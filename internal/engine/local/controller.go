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
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

// A controller that reads CmdSpec and writes CmdStatus
type Controller struct {
	execer        Execer
	procs         map[types.NamespacedName]*currentProcess
	proberManager ProberManager
	updateCmds    map[types.NamespacedName]*Cmd
	mu            sync.Mutex
	client        ctrlclient.Client
}

var _ store.Subscriber = &Controller{}
var _ store.TearDowner = &Controller{}

func NewController(execer Execer, proberManager ProberManager, client ctrlclient.Client) *Controller {
	return &Controller{
		execer:        execer,
		procs:         make(map[types.NamespacedName]*currentProcess),
		proberManager: proberManager,
		client:        client,
		updateCmds:    make(map[types.NamespacedName]*Cmd),
	}
}

func (c *Controller) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) {
	toReconcile := c.diff(ctx, st, summary)
	for name, update := range toReconcile {
		c.reconcile(ctx, st, name, update)
	}
}

func (c *Controller) diff(ctx context.Context, st store.RStore, summary store.ChangeSummary) map[types.NamespacedName]*Cmd {
	state := st.RLockState()
	defer st.RUnlockState()

	result := map[types.NamespacedName]*Cmd{}
	for key := range summary.CmdSpecs {
		result[types.NamespacedName{Name: key}] = state.Cmds[key]
	}
	return result
}

// Stop the command, and wait for it to finish before continuing.
func (c *Controller) stop(name types.NamespacedName) {
	proc, ok := c.procs[name]
	if !ok {
		return
	}

	if proc.cancelFunc == nil {
		return
	}
	proc.cancelFunc()
	<-proc.doneCh
	proc.probeWorker = nil
	proc.cancelFunc = nil
	proc.doneCh = nil
}

func (c *Controller) TearDown(ctx context.Context) {
	for name := range c.procs {
		c.stop(name)
	}
}

func (c *Controller) reconcile(ctx context.Context, st store.RStore, name types.NamespacedName, cmd *Cmd) {
	if cmd == nil || cmd.ObjectMeta.DeletionTimestamp != nil {
		c.stop(name)
		delete(c.procs, name)
		return
	}

	proc, ok := c.procs[name]
	if ok {
		if equality.Semantic.DeepEqual(proc.spec, cmd.Spec) {
			return
		}

		// change the process's current number so that any further events received
		// by the existing process will be considered out of date
		proc.incrProcNum()

		c.stop(name)
	} else {
		proc = &currentProcess{}
		c.procs[name] = proc
	}

	proc.spec = cmd.Spec
	ctx, proc.cancelFunc = context.WithCancel(ctx)

	c.resetStatus(st, name, cmd)

	statusCh := make(chan statusAndMetadata)

	ctx = store.MustObjectLogHandler(ctx, st, cmd)
	spec := cmd.Spec
	spanID := model.LogSpanID(cmd.ObjectMeta.Annotations[v1alpha1.AnnotationSpanID])

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

	serveCmd := model.Cmd{
		Argv: spec.Args,
		Dir:  spec.Dir,
		Env:  spec.Env,
	}
	proc.doneCh = c.execer.Start(ctx, serveCmd, logger.Get(ctx).Writer(logger.InfoLvl), statusCh, spanID)
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

func (c *Controller) resetStatus(st store.RStore, name types.NamespacedName, cmd *Cmd) {
	c.mu.Lock()
	defer c.mu.Unlock()

	updated := cmd.DeepCopy()
	updated.Status = CmdStatus{
		Waiting: &CmdStateWaiting{},
	}
	c.updateCmds[name] = updated

	st.Dispatch(NewCmdUpdateStatusAction(updated))
}

// Update the stored status.
//
// In a real K8s controller, this would be a queue to make sure we don't miss updates.
//
// update() -> a pure function that applies a delta to the status object.
func (c *Controller) updateStatus(st store.RStore, name types.NamespacedName, update func(status *CmdStatus)) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cmd, ok := c.updateCmds[name]
	if !ok {
		return
	}

	update(&cmd.Status)
	st.Dispatch(NewCmdUpdateStatusAction(cmd))
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
	spec       CmdSpec
	procNum    int
	cancelFunc context.CancelFunc
	// closed when the process finishes executing, intentionally or not
	doneCh      chan struct{}
	probeWorker *probe.Worker

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
