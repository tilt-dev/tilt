package local

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/tilt-dev/probe/pkg/probe"
	"github.com/tilt-dev/probe/pkg/prober"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

type Controller struct {
	execer        Execer
	procs         map[model.ManifestName]*currentProcess
	procCount     int
	proberManager ProberManager
	cmds          map[types.NamespacedName]*Cmd
	mu            sync.Mutex
	client        ctrlclient.Client
}

var _ store.Subscriber = &Controller{}
var _ store.TearDowner = &Controller{}

func NewController(execer Execer, proberManager ProberManager, client ctrlclient.Client) *Controller {
	return &Controller{
		execer:        execer,
		procs:         make(map[model.ManifestName]*currentProcess),
		proberManager: proberManager,
		cmds:          make(map[types.NamespacedName]*Cmd),
		client:        client,
	}
}

func (c *Controller) OnChange(ctx context.Context, st store.RStore, _ store.ChangeSummary) {
	specs := c.determineServeSpecs(ctx, st)
	c.update(ctx, specs, st)
}

func (c *Controller) determineServeSpecs(ctx context.Context, st store.RStore) []ServeSpec {
	state := st.RLockState()
	defer st.RUnlockState()

	var r []ServeSpec

	for _, mt := range state.Targets() {
		if !mt.Manifest.IsLocal() {
			continue
		}
		lt := mt.Manifest.LocalTarget()
		if lt.ServeCmd.Empty() ||
			mt.State.LastSuccessfulDeployTime.IsZero() {
			continue
		}
		r = append(r, ServeSpec{
			ManifestName:   mt.Manifest.Name,
			ServeCmd:       lt.ServeCmd,
			TriggerTime:    mt.State.LastSuccessfulDeployTime,
			ReadinessProbe: lt.ReadinessProbe,
		})
	}

	return r
}

func (c *Controller) update(ctx context.Context, specs []ServeSpec, st store.RStore) {
	var toStop []model.ManifestName
	var toStart []ServeSpec

	seen := make(map[model.ManifestName]bool)
	for _, spec := range specs {
		seen[spec.ManifestName] = true
		proc := c.getOrMakeProc(spec.ManifestName)
		if c.shouldStart(spec, proc) {
			toStart = append(toStart, spec)
		}
	}

	for name := range c.procs {
		if !seen[name] {
			toStop = append(toStop, name)
		}
	}

	// stop old ones
	for _, name := range toStop {
		c.stop(name)
		delete(c.procs, name)
	}

	// now start them
	for _, spec := range toStart {
		c.start(ctx, spec, st)
	}
}

func (c *Controller) shouldStart(spec ServeSpec, proc *currentProcess) bool {
	if proc.cancelFunc == nil {
		// nothing is running, so start it
		return true
	}

	if spec.TriggerTime.After(proc.spec.TriggerTime) {
		// there's been a new trigger, so start it
		return true
	}

	return false
}

func (c *Controller) stop(name model.ManifestName) {
	proc := c.procs[name]
	if proc.cmdName.Name != "" {
		c.mu.Lock()
		delete(c.cmds, proc.cmdName)
		c.mu.Unlock()
	}

	// change the process's current number so that any further events received by the existing process will be considered out of date
	proc.incrProcNum()
	if proc.cancelFunc == nil {
		return
	}
	proc.cancelFunc()
	<-proc.doneCh
	proc.probeWorker = nil
	proc.cancelFunc = nil
	proc.doneCh = nil
}

func (c *Controller) getOrMakeProc(name model.ManifestName) *currentProcess {
	if c.procs[name] == nil {
		c.procs[name] = &currentProcess{}
	}

	return c.procs[name]
}

func (c *Controller) TearDown(ctx context.Context) {
	for name := range c.procs {
		c.stop(name)
	}
}

func (c *Controller) start(ctx context.Context, spec ServeSpec, st store.RStore) {
	c.stop(spec.ManifestName)

	proc := c.procs[spec.ManifestName]
	proc.spec = spec
	c.procCount++
	proc.procNum = c.procCount
	ctx, proc.cancelFunc = context.WithCancel(ctx)

	statusCh := make(chan statusAndMetadata)

	cmdName := fmt.Sprintf("%s-serve-%d", spec.ManifestName, c.procCount)
	spanID := SpanIDForServeLog(proc.procNum)
	name := types.NamespacedName{Name: cmdName}
	proc.cmdName = name

	cmd := &Cmd{
		ObjectMeta: ObjectMeta{
			Name: cmdName,
			Annotations: map[string]string{
				v1alpha1.AnnotationSpanID: string(spanID),
			},
			Labels: map[string]string{
				v1alpha1.LabelManifest: spec.ManifestName.String(),
			},
		},
		Spec: CmdSpec{
			Args:           spec.ServeCmd.Argv,
			Dir:            spec.ServeCmd.Dir,
			Env:            spec.ServeCmd.Env,
			ReadinessProbe: spec.ReadinessProbe,
		},
	}
	c.createCmd(st, name, cmd)

	ctx = store.MustObjectLogHandler(ctx, st, cmd)

	if spec.ReadinessProbe != nil {
		statusChangeFunc := c.processReadinessProbeStatusChange(ctx, st, name, proc.stillHasSameProcNum())
		resultLoggerFunc := processReadinessProbeResultLogger(ctx, proc.stillHasSameProcNum())
		probeWorker, err := probeWorkerFromSpec(
			c.proberManager,
			spec.ReadinessProbe,
			statusChangeFunc,
			resultLoggerFunc)
		if err != nil {
			logger.Get(ctx).Errorf("Invalid readiness probe: %v", err)
			c.updateStatus(st, name, func(status *CmdStatus) {
				*status = CmdStatus{
					Terminated: &CmdStateTerminated{
						ExitCode: 1,
						Reason:   fmt.Sprintf("Invalid readiness probe: %v", err),
					},
				}
			})
			return
		}
		proc.probeWorker = probeWorker
	}

	startedAt := metav1.Now()
	go c.processStatuses(ctx, statusCh, st, proc, name, startedAt)

	proc.doneCh = c.execer.Start(ctx, spec.ServeCmd, logger.Get(ctx).Writer(logger.InfoLvl), statusCh, spanID)
}

func (c *Controller) processReadinessProbeStatusChange(ctx context.Context, st store.RStore, name types.NamespacedName, stillHasSameProcNum func() bool) probe.StatusChangedFunc {
	existingReady := false

	return func(status prober.Result, output string) {
		if !stillHasSameProcNum() {
			return
		}

		if status == prober.Success {
			// successful probes are ONLY logged on status change to reduce chattiness
			logProbeOutput(ctx, status, output, nil)
		}

		ready := status == prober.Success || status == prober.Warning
		if existingReady != ready {
			existingReady = ready
			c.updateStatus(st, name, func(status *CmdStatus) { status.Ready = ready })
		}
	}
}

func logProbeOutput(ctx context.Context, result prober.Result, output string, err error) {
	l := logger.Get(ctx)
	if !l.Level().ShouldDisplay(logger.VerboseLvl) {
		return
	}

	if err != nil {
		l.Verbosef("[readiness probe error] %v", err)
	} else if output != "" {
		w := l.Writer(logger.VerboseLvl)
		var logMessage strings.Builder
		s := bufio.NewScanner(strings.NewReader(output))
		for s.Scan() {
			logMessage.WriteString("[readiness probe: ")
			logMessage.WriteString(string(result))
			logMessage.WriteString("] ")
			logMessage.Write(s.Bytes())
			logMessage.WriteRune('\n')
		}
		_, _ = io.WriteString(w, logMessage.String())
	}
}

func processReadinessProbeResultLogger(ctx context.Context, stillHasSameProcNum func() bool) probe.ResultFunc {
	return func(result prober.Result, output string, err error) {
		if !stillHasSameProcNum() {
			return
		}

		// successful probes are ONLY logged on status change to reduce chattiness
		if result != prober.Success {
			logProbeOutput(ctx, result, output, err)
		}
	}
}

// Create the cmd.
func (c *Controller) createCmd(st store.RStore, name types.NamespacedName, cmd *Cmd) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cmds[name] = cmd
	st.Dispatch(NewCmdCreateAction(cmd))
}

// Update the stored status.
//
// In a real K8s controller, this would be a queue to make sure we don't miss updates.
//
// update() -> a pure function that applies a delta to the status object.
func (c *Controller) updateStatus(st store.RStore, name types.NamespacedName, update func(status *CmdStatus)) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cmd, ok := c.cmds[name]
	if !ok {
		return
	}

	update(&cmd.Status)
	st.Dispatch(NewCmdUpdateAction(cmd))
}

func (c *Controller) processStatuses(
	ctx context.Context,
	statusCh chan statusAndMetadata,
	st store.RStore,
	proc *currentProcess,
	name types.NamespacedName,
	startedAt metav1.Time) {

	var initProbeWorker sync.Once
	stillHasSameProcNum := proc.stillHasSameProcNum()

	for sm := range statusCh {
		if !stillHasSameProcNum() || sm.status == Unknown {
			continue
		}

		if sm.status == Error || sm.status == Done {
			c.updateStatus(st, name, func(status *CmdStatus) {
				status.Waiting = nil
				status.Running = nil
				status.Terminated = &CmdStateTerminated{
					PID:        int32(sm.pid),
					Reason:     sm.reason,
					ExitCode:   int32(sm.exitCode),
					StartedAt:  startedAt,
					FinishedAt: metav1.NewTime(time.Now()),
				}
			})
		} else if sm.status == Running {
			if proc.probeWorker != nil {
				initProbeWorker.Do(func() {
					go proc.probeWorker.Run(ctx)
				})
			}

			c.updateStatus(st, name, func(status *CmdStatus) {
				status.Waiting = nil
				status.Running = &CmdStateRunning{
					PID:       int32(sm.pid),
					StartedAt: startedAt,
				}

				if proc.probeWorker == nil {
					status.Ready = true
				}
			})
		}
	}
}

// currentProcess represents the current process for a Manifest, so that Controller can
// make sure there's at most one process per Manifest.
// (note: it may not be running yet, or may have already finished)
type currentProcess struct {
	spec       ServeSpec
	procNum    int
	cancelFunc context.CancelFunc
	// closed when the process finishes executing, intentionally or not
	doneCh      chan struct{}
	probeWorker *probe.Worker
	cmdName     types.NamespacedName

	mu sync.Mutex
}

func (p *currentProcess) stillHasSameProcNum() func() bool {
	s := p.currentProcNum()
	return func() bool {
		return s == p.currentProcNum()
	}
}

func (p *currentProcess) incrProcNum() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.procNum++
}

func (p *currentProcess) currentProcNum() int {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.procNum
}

func SpanIDForServeLog(procNum int) logstore.SpanID {
	return logstore.SpanID(fmt.Sprintf("localserve:%d", procNum))
}

// ServeSpec describes what Runner should be running
type ServeSpec struct {
	ManifestName   model.ManifestName
	ServeCmd       model.Cmd
	TriggerTime    time.Time // TriggerTime is how Runner knows to restart; if it's newer than the TriggerTime of the currently running command, then Runner should restart it
	ReadinessProbe *v1alpha1.Probe
}

type statusAndMetadata struct {
	pid      int
	status   status
	spanID   model.LogSpanID
	exitCode int
	reason   string
}

type status int

const (
	Unknown status = iota
	Running status = iota
	Done    status = iota
	Error   status = iota
)
