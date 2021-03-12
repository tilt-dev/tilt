package cmd

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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/engine/local"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

// A controller that reads CmdSpec and writes CmdStatus
type Controller struct {
	globalCtx     context.Context
	execer        Execer
	procs         map[types.NamespacedName]*currentProcess
	proberManager ProberManager
	updateCmds    map[types.NamespacedName]*Cmd
	mu            sync.Mutex
	client        ctrlclient.Client
	st            store.RStore
}

var _ store.TearDowner = &Controller{}

func NewController(ctx context.Context, execer Execer, proberManager ProberManager, client ctrlclient.Client, st store.RStore) *Controller {
	return &Controller{
		globalCtx:     ctx,
		execer:        execer,
		procs:         make(map[types.NamespacedName]*currentProcess),
		proberManager: proberManager,
		client:        client,
		updateCmds:    make(map[types.NamespacedName]*Cmd),
		st:            st,
	}
}

func (c *Controller) SetClient(client ctrlclient.Client) {
	c.client = client
}

func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	err := c.reconcile(ctx, req.NamespacedName)
	return ctrl.Result{}, err
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

func (c *Controller) reconcile(ctx context.Context, name types.NamespacedName) error {
	cmd := &Cmd{}
	err := c.client.Get(ctx, name, cmd)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("cmd reconcile: %v", err)
	}

	if apierrors.IsNotFound(err) || cmd.ObjectMeta.DeletionTimestamp != nil {
		c.stop(name)
		delete(c.procs, name)
		return nil
	}

	proc, ok := c.procs[name]
	if ok {
		if equality.Semantic.DeepEqual(proc.spec, cmd.Spec) {
			return nil
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

	c.resetStatus(name, cmd)

	statusCh := make(chan statusAndMetadata)

	ctx = store.MustObjectLogHandler(ctx, c.st, cmd)
	spec := cmd.Spec
	spanID := model.LogSpanID(cmd.ObjectMeta.Annotations[v1alpha1.AnnotationSpanID])

	if spec.ReadinessProbe != nil {
		statusChangeFunc := c.processReadinessProbeStatusChange(ctx, name, proc.stillHasSameProcNum())
		resultLoggerFunc := processReadinessProbeResultLogger(ctx, proc.stillHasSameProcNum())
		probeWorker, err := probeWorkerFromSpec(
			c.proberManager,
			spec.ReadinessProbe,
			statusChangeFunc,
			resultLoggerFunc)
		if err != nil {
			logger.Get(ctx).Errorf("Invalid readiness probe: %v", err)
			c.updateStatus(name, func(status *CmdStatus) {
				*status = CmdStatus{
					Terminated: &CmdStateTerminated{
						ExitCode: 1,
						Reason:   fmt.Sprintf("Invalid readiness probe: %v", err),
					},
				}
			})
			return nil
		}
		proc.probeWorker = probeWorker
	}

	startedAt := metav1.Now()
	go c.processStatuses(ctx, statusCh, proc, name, startedAt)

	serveCmd := model.Cmd{
		Argv: spec.Args,
		Dir:  spec.Dir,
		Env:  spec.Env,
	}
	proc.doneCh = c.execer.Start(ctx, serveCmd, logger.Get(ctx).Writer(logger.InfoLvl), statusCh, spanID)
	return nil
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
			c.updateStatus(name, func(status *CmdStatus) { status.Ready = ready })
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

func (c *Controller) resetStatus(name types.NamespacedName, cmd *Cmd) {
	c.mu.Lock()
	defer c.mu.Unlock()

	updated := cmd.DeepCopy()
	updated.Status = CmdStatus{
		Waiting: &CmdStateWaiting{},
	}
	c.updateCmds[name] = updated

	err := c.client.Status().Update(c.globalCtx, updated)
	if err != nil && !apierrors.IsNotFound(err) {
		c.st.Dispatch(store.NewErrorAction(fmt.Errorf("syncing to apiserver: %v", err)))
		return
	}

	c.st.Dispatch(local.NewCmdUpdateStatusAction(updated))
}

// Update the stored status.
//
// In a real K8s controller, this would be a queue to make sure we don't miss updates.
//
// update() -> a pure function that applies a delta to the status object.
func (c *Controller) updateStatus(name types.NamespacedName, update func(status *CmdStatus)) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cmd, ok := c.updateCmds[name]
	if !ok {
		return
	}

	update(&cmd.Status)

	err := c.client.Status().Update(c.globalCtx, cmd)
	if err != nil && !apierrors.IsNotFound(err) {
		c.st.Dispatch(store.NewErrorAction(fmt.Errorf("syncing to apiserver: %v", err)))
		return
	}

	c.st.Dispatch(local.NewCmdUpdateStatusAction(cmd))
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
			c.updateStatus(name, func(status *CmdStatus) {
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

			c.updateStatus(name, func(status *CmdStatus) {
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

func (r *Controller) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&Cmd{}).
		Complete(r)
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
