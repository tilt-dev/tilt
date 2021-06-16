package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/tilt-dev/probe/pkg/probe"
	"github.com/tilt-dev/probe/pkg/prober"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/engine/local"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

// A controller that reads CmdSpec and writes CmdStatus
type Controller struct {
	globalCtx     context.Context
	indexer       *indexer.Indexer
	execer        Execer
	procs         map[types.NamespacedName]*currentProcess
	proberManager ProberManager
	updateCmds    map[types.NamespacedName]*Cmd
	mu            sync.Mutex
	client        ctrlclient.Client
	st            store.RStore
	clock         clockwork.Clock
}

var _ store.TearDowner = &Controller{}

func NewController(ctx context.Context, execer Execer, proberManager ProberManager, client ctrlclient.Client, st store.RStore, clock clockwork.Clock, scheme *runtime.Scheme) *Controller {
	return &Controller{
		globalCtx:     ctx,
		indexer:       indexer.NewIndexer(scheme, indexCmd),
		clock:         clock,
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

// Fetch the last time a start was requested from this target's dependencies.
func (c *Controller) lastStartEventTime(ctx context.Context, startOn *StartOnSpec) (time.Time, error) {
	cur := time.Time{}
	if startOn == nil {
		return cur, nil
	}

	for _, bn := range startOn.UIButtons {
		b := &UIButton{}
		err := c.client.Get(ctx, types.NamespacedName{Name: bn}, b)
		if err != nil {
			return cur, err
		}
		lastEventTime := b.Status.LastClickedAt
		if !lastEventTime.Time.Before(startOn.StartAfter.Time) && lastEventTime.Time.After(cur) {
			cur = lastEventTime.Time
		}
	}
	return cur, nil
}

// Fetch the last time a restart was requested from this target's dependencies.
func (c *Controller) lastRestartEventTime(ctx context.Context, restartOn *RestartOnSpec) (time.Time, error) {
	cur := time.Time{}
	if restartOn == nil {
		return cur, nil
	}

	for _, fwn := range restartOn.FileWatches {
		fw := &FileWatch{}
		err := c.client.Get(ctx, types.NamespacedName{Name: fwn}, fw)
		if err != nil {
			return cur, err
		}
		lastEventTime := fw.Status.LastEventTime
		if lastEventTime.Time.After(cur) {
			cur = lastEventTime.Time
		}
	}
	return cur, nil
}

func (c *Controller) reconcile(ctx context.Context, name types.NamespacedName) error {
	cmd := &Cmd{}
	err := c.client.Get(ctx, name, cmd)
	c.indexer.OnReconcile(name, cmd)
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("cmd reconcile: %v", err)
	}

	if apierrors.IsNotFound(err) || cmd.ObjectMeta.DeletionTimestamp != nil {
		c.stop(name)
		delete(c.procs, name)
		return nil
	}

	lastRestartEventTime, err := c.lastRestartEventTime(ctx, cmd.Spec.RestartOn)
	if err != nil {
		return err
	}
	lastStartEventTime, err := c.lastStartEventTime(ctx, cmd.Spec.StartOn)
	if err != nil {
		return err
	}

	proc, ok := c.procs[name]
	// there is already a proc (which may or may not be running) for this cmd
	if ok {
		// any change to the spec means we should restart the process to pick up the changes
		specChanged := !equality.Semantic.DeepEqual(proc.spec, cmd.Spec)
		// a new restart event happened
		restartOnTriggered := lastRestartEventTime.After(proc.lastRestartOnEventTime)
		// a new start event happened
		startOnTriggered := lastStartEventTime.After(proc.lastStartOnEventTime)
		needsRestart := specChanged || restartOnTriggered || startOnTriggered
		if !needsRestart {
			return nil
		}

		// change the process's current number so that any further events received
		// by the existing process will be considered out of date
		proc.incrProcNum()

		c.stop(name)
	} else {
		startOn := cmd.Spec.StartOn
		startOnSpecIsEmpty := startOn == nil || len(startOn.UIButtons) == 0
		// cmds with non-empty StartOn specs don't start until triggered
		if !startOnSpecIsEmpty && lastStartEventTime.IsZero() {
			c.setStatusWaitingOnStartOn(cmd)
			return nil
		}
		proc = &currentProcess{}
		c.procs[name] = proc
	}

	proc.spec = cmd.Spec
	proc.isServer = cmd.ObjectMeta.Annotations[local.AnnotationOwnerKind] == "CmdServer"
	proc.lastRestartOnEventTime = lastRestartEventTime
	proc.lastStartOnEventTime = lastStartEventTime
	ctx, proc.cancelFunc = context.WithCancel(ctx)

	c.resetStatus(name, cmd)

	stillHasSameProcNum := proc.stillHasSameProcNum()
	statusCh := make(chan statusAndMetadata)

	ctx = store.MustObjectLogHandler(ctx, c.st, cmd)
	spec := cmd.Spec
	spanID := model.LogSpanID(cmd.ObjectMeta.Annotations[v1alpha1.AnnotationSpanID])

	if spec.ReadinessProbe != nil {
		statusChangeFunc := c.processReadinessProbeStatusChange(ctx, name, stillHasSameProcNum)
		resultLoggerFunc := processReadinessProbeResultLogger(ctx, stillHasSameProcNum)
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
			}, stillHasSameProcNum)
			return nil
		}
		proc.probeWorker = probeWorker
	}

	startedAt := apis.NewMicroTime(c.clock.Now())
	go c.processStatuses(ctx, statusCh, proc, name, startedAt, stillHasSameProcNum)

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
			c.updateStatus(name, func(status *CmdStatus) { status.Ready = ready }, stillHasSameProcNum)
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
func (c *Controller) updateStatus(name types.NamespacedName, update func(status *CmdStatus), stillHasSameProcNum func() bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !stillHasSameProcNum() {
		return
	}

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

const waitingOnStartOnReason = "cmd StartOn has not been triggered"

func (c *Controller) setStatusWaitingOnStartOn(cmd *Cmd) {
	if cmd.Status.Waiting != nil && cmd.Status.Waiting.Reason == waitingOnStartOnReason {
		return
	}

	updated := cmd.DeepCopy()
	updated.Status = CmdStatus{
		Waiting: &CmdStateWaiting{
			Reason: waitingOnStartOnReason,
		},
	}

	err := c.client.Status().Update(c.globalCtx, updated)
	if err != nil && !apierrors.IsNotFound(err) {
		c.st.Dispatch(store.NewErrorAction(fmt.Errorf("syncing to apiserver: %v", err)))
		return
	}

	c.st.Dispatch(local.NewCmdUpdateStatusAction(updated))
}

func (c *Controller) processStatuses(
	ctx context.Context,
	statusCh chan statusAndMetadata,
	proc *currentProcess,
	name types.NamespacedName,
	startedAt metav1.MicroTime,
	stillHasSameProcNum func() bool) {

	var initProbeWorker sync.Once

	for sm := range statusCh {
		if !stillHasSameProcNum() || sm.status == Unknown {
			continue
		}

		if sm.status == Error || sm.status == Done {
			// This is a hack until CmdServer is a real object.
			if proc.isServer && sm.exitCode == 0 {
				logger.Get(ctx).Errorf("Server exited with exit code 0")
			}

			c.updateStatus(name, func(status *CmdStatus) {
				status.Waiting = nil
				status.Running = nil
				status.Terminated = &CmdStateTerminated{
					PID:        int32(sm.pid),
					Reason:     sm.reason,
					ExitCode:   int32(sm.exitCode),
					StartedAt:  startedAt,
					FinishedAt: metav1.NowMicro(),
				}
			}, stillHasSameProcNum)
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
			}, stillHasSameProcNum)
		}
	}
}

// Find all the objects we need to watch based on the Cmd model.
func indexCmd(obj client.Object) []indexer.Key {
	cmd := obj.(*v1alpha1.Cmd)
	result := []indexer.Key{}
	if cmd.Spec.StartOn != nil {
		bGVK := v1alpha1.SchemeGroupVersion.WithKind("UIButton")

		for _, name := range cmd.Spec.StartOn.UIButtons {
			result = append(result, indexer.Key{
				Name: types.NamespacedName{Name: name},
				GVK:  bGVK,
			})
		}
	}

	if cmd.Spec.RestartOn != nil {
		fwGVK := v1alpha1.SchemeGroupVersion.WithKind("FileWatch")

		for _, name := range cmd.Spec.RestartOn.FileWatches {
			result = append(result, indexer.Key{
				Name: types.NamespacedName{Name: name},
				GVK:  fwGVK,
			})
		}
	}
	return result
}

func (r *Controller) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&Cmd{}).
		Watches(&source.Kind{Type: &v1alpha1.FileWatch{}},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue)).
		Watches(&source.Kind{Type: &v1alpha1.UIButton{}},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue)).
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
	isServer    bool

	lastRestartOnEventTime time.Time
	lastStartOnEventTime   time.Time

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
