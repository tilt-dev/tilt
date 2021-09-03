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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/tilt-dev/probe/pkg/probe"
	"github.com/tilt-dev/probe/pkg/prober"

	"github.com/tilt-dev/tilt/internal/controllers/apis/restarton"
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
	return restarton.Buttons(ctx, c.client, cmd.Spec.RestartOn, cmd.Spec.StartOn)
}

// Fetch all the filewatches that this object depends on.
func (c *Controller) fileWatches(ctx context.Context, cmd *v1alpha1.Cmd) (map[string]*v1alpha1.FileWatch, error) {
	return restarton.FileWatches(ctx, c.client, cmd.Spec.RestartOn)
}

func inputsFromButton(button v1alpha1.UIButton) []input {
	statuses := make(map[string]v1alpha1.UIInputStatus)
	for _, status := range button.Status.Inputs {
		statuses[status.Name] = status
	}

	var ret []input
	for _, spec := range button.Spec.Inputs {
		ret = append(ret, input{
			spec:   spec,
			status: statuses[spec.Name],
		})
	}

	return ret
}

// Fetch the last time a start was requested from this target's dependencies.
func (c *Controller) lastStartEvent(startOn *StartOnSpec, buttons map[string]*v1alpha1.UIButton) (time.Time, []input) {
	latestTime, latestButton := restarton.LastStartEvent(startOn, buttons)
	var inputs []input
	if latestButton != nil {
		inputs = inputsFromButton(*latestButton)
	}
	return latestTime, inputs
}

// Fetch the last time a restart was requested from this target's dependencies.
func (c *Controller) lastRestartEvent(restartOn *RestartOnSpec, fileWatches map[string]*v1alpha1.FileWatch, buttons map[string]*v1alpha1.UIButton) (time.Time, []input) {
	cur, latestButton := restarton.LastRestartEvent(restartOn, fileWatches, buttons)
	var inputs []input
	if latestButton != nil {
		inputs = inputsFromButton(*latestButton)
	}
	return cur, inputs
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

	owner := metav1.GetControllerOf(cmd)
	if owner != nil && owner.Kind == v1alpha1.OwnerKindTiltfile {
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

	lastRestartEventTime, _ := c.lastRestartEvent(cmd.Spec.RestartOn, fileWatches, buttons)
	lastStartEventTime, _ := c.lastStartEvent(cmd.Spec.StartOn, buttons)
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

func (i input) stringValue() string {
	if i.status.Text != nil {
		return i.status.Text.Value
	} else if i.status.Bool != nil {
		if i.status.Bool.Value {
			if i.spec.Bool.TrueString != nil {
				return *i.spec.Bool.TrueString
			} else {
				return "true"
			}
		} else {
			if i.spec.Bool.FalseString != nil {
				return *i.spec.Bool.FalseString
			} else {
				return "false"
			}
		}
	}
	return ""
}

type input struct {
	spec   v1alpha1.UIInputSpec
	status v1alpha1.UIInputStatus
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

	var startInputs, restartInputs []input

	proc.lastRestartOnEventTime, restartInputs = c.lastRestartEvent(cmd.Spec.RestartOn, fileWatches, buttons)
	proc.lastStartOnEventTime, startInputs = c.lastStartEvent(cmd.Spec.StartOn, buttons)

	mergedInputs := startInputs
	if proc.lastRestartOnEventTime.After(proc.lastStartOnEventTime) {
		mergedInputs = restartInputs
	}

	ctx, proc.cancelFunc = context.WithCancel(ctx)

	stillHasSameProcNum := proc.stillHasSameProcNum()
	c.updateStatus(name, func(status *CmdStatus) {
		*status = CmdStatus{Waiting: &CmdStateWaiting{}}
	}, stillHasSameProcNum)

	ctx = store.MustObjectLogHandler(ctx, c.st, cmd)
	spec := cmd.Spec

	if spec.ReadinessProbe != nil {
		probeResultFunc := c.handleProbeResultFunc(ctx, name, stillHasSameProcNum)
		probeWorker, err := probeWorkerFromSpec(
			c.proberManager,
			spec.ReadinessProbe,
			probeResultFunc)
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

	env := append([]string{}, spec.Env...)
	for _, input := range mergedInputs {
		env = append(env, fmt.Sprintf("%s=%s", input.spec.Name, input.stringValue()))
	}

	cmdModel := model.Cmd{
		Argv: spec.Args,
		Dir:  spec.Dir,
		Env:  env,
	}
	statusCh := c.execer.Start(ctx, cmdModel, logger.Get(ctx).Writer(logger.InfoLvl))
	proc.doneCh = make(chan struct{})

	go c.processStatuses(ctx, statusCh, proc, name, startedAt, stillHasSameProcNum)

	return proc.doneCh
}

func (c *Controller) handleProbeResultFunc(ctx context.Context, name types.NamespacedName, stillHasSameProcNum func() bool) probe.ResultFunc {
	existingReady := false

	return func(result prober.Result, statusChanged bool, output string, err error) {
		if !stillHasSameProcNum() {
			return
		}

		// we try to balance logging important probe results without flooding the logs
		//  * ALL transitions are logged
		// 		* success->{failure,warning} @ WARN
		// 		* {failure,warning}->success @ INFO
		// 	* subsequent non-successful results @ VERBOSE
		// 		* expected healthy/steady-state is recurring success, and this is apparent
		// 		  from the "Ready" state, so logging every invocation is superfluous
		loggerLevel := logger.NoneLvl
		if statusChanged {
			if result != prober.Success {
				loggerLevel = logger.WarnLvl
			} else {
				loggerLevel = logger.InfoLvl
			}
		} else if result != prober.Success {
			loggerLevel = logger.VerboseLvl
		}
		logProbeOutput(ctx, loggerLevel, result, output, nil)

		if !statusChanged {
			// the probe did not transition states, so the result is logged but not used to update status
			return
		}

		ready := result == prober.Success || result == prober.Warning
		if existingReady != ready {
			existingReady = ready
			c.updateStatus(name, func(status *CmdStatus) { status.Ready = ready }, stillHasSameProcNum)
		}
	}
}

func logProbeOutput(ctx context.Context, level logger.Level, result prober.Result, output string, err error) {
	l := logger.Get(ctx)
	if level == logger.NoneLvl || !l.Level().ShouldDisplay(level) {
		return
	}

	w := l.Writer(level)
	if err != nil {
		_, _ = fmt.Fprintf(w, "[readiness probe error] %v\n", err)
	} else if output != "" {
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

		bGVK := v1alpha1.SchemeGroupVersion.WithKind("UIButton")

		for _, name := range cmd.Spec.RestartOn.UIButtons {
			result = append(result, indexer.Key{
				Name: types.NamespacedName{Name: name},
				GVK:  bGVK,
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
