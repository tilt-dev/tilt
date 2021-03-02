package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"github.com/tilt-dev/probe/pkg/probe"
	"github.com/tilt-dev/probe/pkg/prober"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

type Controller struct {
	client.Client

	st            store.RStore
	execer        Execer
	procs         map[types.NamespacedName]*currentProcess
	procCount     int
	proberManager ProberManager
	mu            sync.Mutex
}

func NewController(st store.RStore, execer Execer, proberManager ProberManager) *Controller {
	return &Controller{
		st:            st,
		execer:        execer,
		proberManager: proberManager,
		procs:         make(map[types.NamespacedName]*currentProcess),
	}
}

func (c *Controller) SetClient(client client.Client) {
	c.Client = client
}

func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log.Println("RECONCILE", req)
	var cmd Cmd
	err := c.Get(ctx, req.NamespacedName, &cmd)
	if err != nil {
		if apierrors.IsNotFound(err) {
			c.stop(req.NamespacedName)
		}
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !cmd.ObjectMeta.DeletionTimestamp.IsZero() {
		c.stop(req.NamespacedName)
		return ctrl.Result{}, nil
	}

	c.ensureStarted(ctx, req.NamespacedName, &cmd)

	return ctrl.Result{}, nil
}

func (c *Controller) stop(name types.NamespacedName) {
	proc, exists := c.procs[name]
	if !exists {
		return
	}

	delete(c.procs, name)

	// change the process's current number so that any further events received by the existing process will be considered out of date
	proc.incrProcNum()
	if proc.cancelFunc == nil {
		return
	}
	proc.cancelFunc()

	if proc.doneCh != nil {
		<-proc.doneCh
	}

	proc.probeWorker = nil
	proc.cancelFunc = nil
	proc.doneCh = nil
}

func (c *Controller) TearDown(ctx context.Context) {
	for name := range c.procs {
		c.stop(name)
	}
}

func (c *Controller) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&Cmd{}).
		Complete(c)
}

func (c *Controller) ensureStarted(ctx context.Context, name types.NamespacedName, cmd *Cmd) {
	existing, exists := c.procs[name]
	if exists && equality.Semantic.DeepEqual(existing.spec, cmd.Spec) {
		return
	}

	cmd = cmd.DeepCopy()

	if exists {
		c.stop(name)

		// reset the status
		c.updateStatus(ctx, name, func(status *CmdStatus) { *status = CmdStatus{} })
	}

	proc := &currentProcess{}
	c.procs[name] = proc
	proc.spec = cmd.Spec
	c.procCount++
	proc.procNum = c.procCount
	ctx, proc.cancelFunc = context.WithCancel(ctx)

	manifestName := model.ManifestName(cmd.ObjectMeta.Labels[LabelManifest])
	w := CmdLogActionWriter{
		store:        c.st,
		manifestName: manifestName,
		procNum:      proc.procNum,
	}
	ctx = logger.CtxWithLogHandler(ctx, w)

	statusCh := make(chan statusAndMetadata)

	spanID := SpanIDForServeLog(proc.procNum)
	if cmd.Spec.ReadinessProbe != nil {
		statusChangeFunc := c.processReadinessProbeStatusChange(ctx, name, proc.stillHasSameProcNum())
		resultLoggerFunc := processReadinessProbeResultLogger(ctx, proc.stillHasSameProcNum())
		probeWorker, err := probeWorkerFromSpec(
			c.proberManager,
			cmd.Spec.ReadinessProbe,
			statusChangeFunc,
			resultLoggerFunc)
		if err != nil {
			logger.Get(ctx).Errorf("Invalid readiness probe: %v", err)
			c.updateStatus(ctx, name, func(status *CmdStatus) {
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
	} else {
		c.updateStatus(ctx, name, func(status *CmdStatus) { status.Ready = true })
	}

	startedAt := metav1.NewTime(time.Now())

	go c.processStatuses(ctx, statusCh, proc, name, startedAt)

	modelCmd := model.Cmd{
		Argv: cmd.Spec.Args,
		Dir:  cmd.Spec.Dir,
		Env:  cmd.Spec.Env,
	}
	proc.doneCh = c.execer.Start(ctx, modelCmd, logger.Get(ctx).Writer(logger.InfoLvl), statusCh, spanID)
}

func (c *Controller) processReadinessProbeStatusChange(ctx context.Context, name types.NamespacedName, stillHasSameProcNum func() bool) probe.StatusChangedFunc {
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
			c.updateStatus(ctx, name, func(status *CmdStatus) { status.Ready = ready })
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

// Update the stored status.
//
// In a real K8s controller, this would be a queue to make sure we don't miss updates.
//
// update() -> a pure function that applies a delta to the status object.
func (c *Controller) updateStatus(ctx context.Context, name types.NamespacedName, update func(status *CmdStatus)) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var cmd Cmd
	err := c.Get(ctx, name, &cmd)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return
		}
		logger.Get(ctx).Infof("cmd status update: %v", err)
	}

	proc, ok := c.procs[name]
	if !ok {
		return
	}

	newStatus := proc.status
	update(&newStatus)
	proc.status = newStatus

	updated := cmd.DeepCopy()
	updated.Status = *(newStatus.DeepCopy())

	err = c.Status().Update(ctx, updated)
	if err != nil {
		logger.Get(ctx).Infof("cmd status update: %v", err)
	}
	c.st.Dispatch(store.NewCmdUpdateAction(updated))
}

func (c *Controller) processStatuses(
	ctx context.Context,
	statusCh chan statusAndMetadata,
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
			c.updateStatus(ctx, name, func(status *CmdStatus) {
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
			c.updateStatus(ctx, name, func(status *CmdStatus) {
				status.Waiting = nil
				status.Running = &CmdStateRunning{
					PID:       int32(sm.pid),
					StartedAt: startedAt,
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
	status     CmdStatus
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

type CmdLogActionWriter struct {
	store        store.RStore
	manifestName model.ManifestName
	procNum      int
}

func (w CmdLogActionWriter) Write(level logger.Level, fields logger.Fields, p []byte) error {
	w.store.Dispatch(store.NewLogAction(w.manifestName, SpanIDForServeLog(w.procNum), level, fields, p))
	return nil
}

func SpanIDForServeLog(procNum int) logstore.SpanID {
	return logstore.SpanID(fmt.Sprintf("cmd:%d", procNum))
}

type statusAndMetadata struct {
	pid      int
	status   status
	spanID   model.LogSpanID
	reason   string
	exitCode int
}

type status int

const (
	Unknown status = iota
	Running status = iota
	Done    status = iota
	Error   status = iota
)
