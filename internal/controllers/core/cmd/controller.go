package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/builder"

	"github.com/jonboulle/clockwork"
	"github.com/tilt-dev/probe/pkg/probe"
	"github.com/tilt-dev/probe/pkg/prober"
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
	client        ctrlclient.Client
	st            store.RStore
	clock         clockwork.Clock

	// Ensures that we're only updating one Cmd Status at a time.
	updateMu sync.Mutex

	// Ensures that we're only reonciling one Cmd at a time.
	reconcileMu sync.Mutex
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

func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	c.reconcileMu.Lock()
	defer c.reconcileMu.Unlock()
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

// Fetch all the buttons that this object depends on.
func (c *Controller) buttons(ctx context.Context, cmd *v1alpha1.Cmd) (map[string]*v1alpha1.UIButton, error) {
	startOn := cmd.Spec.StartOn
	if startOn == nil {
		return nil, nil
	}

	result := make(map[string]*v1alpha1.UIButton, len(startOn.UIButtons))
	for _, n := range startOn.UIButtons {
		b := &UIButton{}
		err := c.client.Get(ctx, types.NamespacedName{Name: n}, b)
		if err != nil {
			return nil, err
		}
		result[n] = b
	}
	return result, nil
}

// Fetch all the filewatches that this object depends on.
func (c *Controller) fileWatches(ctx context.Context, cmd *v1alpha1.Cmd) (map[string]*v1alpha1.FileWatch, error) {
	restartOn := cmd.Spec.RestartOn
	if restartOn == nil {
		return nil, nil
	}

	result := make(map[string]*v1alpha1.FileWatch, len(restartOn.FileWatches))
	for _, n := range restartOn.FileWatches {
		fw := &v1alpha1.FileWatch{}
		err := c.client.Get(ctx, types.NamespacedName{Name: n}, fw)
		if err != nil {
			return nil, err
		}
		result[n] = fw
	}
	return result, nil
}

// Fetch the last time a start was requested from this target's dependencies.
func (c *Controller) lastStartEventTime(startOn *StartOnSpec, buttons map[string]*v1alpha1.UIButton) time.Time {
	cur := time.Time{}
	if startOn == nil {
		return cur
	}

	for _, bn := range startOn.UIButtons {
		b, ok := buttons[bn]
		if !ok {
			// ignore missing buttons
			continue
		}
		lastEventTime := b.Status.LastClickedAt
		if !lastEventTime.Time.Before(startOn.StartAfter.Time) && lastEventTime.Time.After(cur) {
			cur = lastEventTime.Time
		}
	}
	return cur
}

// Fetch the last time a restart was requested from this target's dependencies.
func (c *Controller) lastRestartEventTime(restartOn *RestartOnSpec, fileWatches map[string]*v1alpha1.FileWatch) time.Time {
	cur := time.Time{}
	if restartOn == nil {
		return cur
	}

	for _, fwn := range restartOn.FileWatches {
		fw, ok := fileWatches[fwn]
		if !ok {
			// ignore missing filewatches
			continue
		}
		lastEventTime := fw.Status.LastEventTime
		if lastEventTime.Time.After(cur) {
			cur = lastEventTime.Time
		}
	}
	return cur
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

	if cmd.ObjectMeta.Labels[v1alpha1.LabelOwnerKind] == v1alpha1.LabelOwnerKindTiltfile {
		// Until resource dependencies are expressed in the API,
		// we can't use reconciliation to deploy Cmd objects
		// owned by the Tiltfile.
		return nil
	}

	buttons, err := c.buttons(ctx, cmd)
	if err != nil {
		return err
	}

	fileWatches, err := c.fileWatches(ctx, cmd)
	if err != nil {
		return err
	}

	lastRestartEventTime := c.lastRestartEventTime(cmd.Spec.RestartOn, fileWatches)
	lastStartEventTime := c.lastStartEventTime(cmd.Spec.StartOn, buttons)

	startOn := cmd.Spec.StartOn
	waitsOnStartOn := startOn != nil && len(startOn.UIButtons) > 0

	proc, hasExistingProc := c.procs[name]

	lastSpec := v1alpha1.CmdSpec{}
	lastRestartOnEventTime := time.Time{}
	lastStartOnEventTime := time.Time{}
	if hasExistingProc {
		lastSpec = proc.spec
		lastRestartOnEventTime = proc.lastRestartOnEventTime
		lastStartOnEventTime = proc.lastStartOnEventTime
	}

	restartOnTriggered := lastRestartEventTime.After(lastRestartOnEventTime)
	startOnTriggered := lastStartEventTime.After(lastStartOnEventTime)
	execSpecChanged := !cmdExecEqual(lastSpec, cmd.Spec)

	// any change to the spec means we should stop the command immediately
	if execSpecChanged {
		c.stop(name)
	}

	if execSpecChanged && waitsOnStartOn && !startOnTriggered {
		// If the cmd spec has changed since the last run,
		// and StartOn hasn't triggered yet, set the status to waiting.
		c.setStatusWaitingOnStartOn(name, cmd)
	} else if execSpecChanged || restartOnTriggered || startOnTriggered {
		// Otherwise, any change, new start event, or new restart event
		// should restart the process to pick up changes.
		_ = c.runInternal(ctx, cmd, buttons, fileWatches)
	}

	return nil
}

// Forces the command to run now.
//
// This is a hack to get local_resource commands into the API server,
// even though the API server doesn't have a notion of resource deps yet.
//
// Blocks until the command is finished, then returns its status.
func (c *Controller) ForceRun(ctx context.Context, cmd *v1alpha1.Cmd) (*v1alpha1.CmdStatus, error) {
	c.reconcileMu.Lock()
	doneCh := c.runInternal(ctx, cmd, nil, nil)
	c.reconcileMu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-doneCh:
	}

	c.updateMu.Lock()
	defer c.updateMu.Unlock()

	result, ok := c.updateCmds[types.NamespacedName{Name: cmd.Name}]
	if !ok {
		return nil, fmt.Errorf("internal error: no cmd status")
	}

	return result.Status.DeepCopy(), nil
}

// Runs the command unconditionally, stopping any currently running command.
//
// The filewatches and buttons are needed for bookkeeping on how the command
// was triggered.
//
// Returns a channel that closes when the Cmd is finished.
func (c *Controller) runInternal(ctx context.Context,
	cmd *v1alpha1.Cmd,
	buttons map[string]*v1alpha1.UIButton,
	fileWatches map[string]*v1alpha1.FileWatch) (doneCh chan struct{}) {
	name := types.NamespacedName{Name: cmd.Name}
	proc, ok := c.procs[name]
	if ok {
		// change the process's current number so that any further events received
		// by the existing process will be considered out of date
		proc.incrProcNum()

		c.stop(name)
	} else {
		proc = &currentProcess{}
		c.procs[name] = proc
	}

	proc.spec = cmd.Spec
	proc.isServer = cmd.ObjectMeta.Annotations[local.AnnotationOwnerKind] == "CmdServer"
	proc.lastRestartOnEventTime = c.lastRestartEventTime(cmd.Spec.RestartOn, fileWatches)
	proc.lastStartOnEventTime = c.lastStartEventTime(cmd.Spec.StartOn, buttons)
	ctx, proc.cancelFunc = context.WithCancel(ctx)

	stillHasSameProcNum := proc.stillHasSameProcNum()
	c.updateStatus(name, func(status *CmdStatus) {
		*status = CmdStatus{Waiting: &CmdStateWaiting{}}
	}, stillHasSameProcNum)

	ctx = store.MustObjectLogHandler(ctx, c.st, cmd)
	spec := cmd.Spec

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

			proc.doneCh = make(chan struct{})
			close(proc.doneCh)
			return proc.doneCh
		}
		proc.probeWorker = probeWorker
	}

	startedAt := apis.NewMicroTime(c.clock.Now())

	cmdModel := model.Cmd{
		Argv: spec.Args,
		Dir:  spec.Dir,
		Env:  spec.Env,
	}
	statusCh := c.execer.Start(ctx, cmdModel, logger.Get(ctx).Writer(logger.InfoLvl))
	proc.doneCh = make(chan struct{})

	go c.processStatuses(ctx, statusCh, proc, name, startedAt, stillHasSameProcNum)

	return proc.doneCh
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

// Update the stored status.
//
// In a real K8s controller, this would be a queue to make sure we don't miss updates.
//
// update() -> a pure function that applies a delta to the status object.
func (c *Controller) updateStatus(name types.NamespacedName, update func(status *CmdStatus), stillHasSameProcNum func() bool) {
	c.updateMu.Lock()
	defer c.updateMu.Unlock()

	if !stillHasSameProcNum() {
		return
	}

	var cmd v1alpha1.Cmd
	err := c.client.Get(c.globalCtx, name, &cmd)
	if err != nil {
		return
	}

	lastCmd, ok := c.updateCmds[name]
	if ok {
		cmd.Status = lastCmd.Status
	}
	update(&cmd.Status)
	c.updateCmds[name] = cmd.DeepCopy()

	err = c.client.Status().Update(c.globalCtx, &cmd)
	if err != nil && !apierrors.IsNotFound(err) {
		c.st.Dispatch(store.NewErrorAction(fmt.Errorf("syncing to apiserver: %v", err)))
		return
	}

	c.st.Dispatch(local.NewCmdUpdateStatusAction(&cmd))
}

const waitingOnStartOnReason = "cmd StartOn has not been triggered"

func (c *Controller) setStatusWaitingOnStartOn(name types.NamespacedName, cmd *Cmd) {
	if cmd.Status.Waiting != nil && cmd.Status.Waiting.Reason == waitingOnStartOnReason {
		return
	}
	c.updateStatus(name, func(status *CmdStatus) {
		*status = CmdStatus{
			Waiting: &CmdStateWaiting{
				Reason: waitingOnStartOnReason,
			},
		}
	}, func() bool { return true })
}

func (c *Controller) processStatuses(
	ctx context.Context,
	statusCh chan statusAndMetadata,
	proc *currentProcess,
	name types.NamespacedName,
	startedAt metav1.MicroTime,
	stillHasSameProcNum func() bool) {
	defer close(proc.doneCh)

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

func (r *Controller) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&Cmd{}).
		Watches(&source.Kind{Type: &v1alpha1.FileWatch{}},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue)).
		Watches(&source.Kind{Type: &v1alpha1.UIButton{}},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue))

	return b, nil
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
