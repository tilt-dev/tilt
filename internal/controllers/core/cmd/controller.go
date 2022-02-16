package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/jonboulle/clockwork"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/tilt-dev/probe/pkg/probe"
	"github.com/tilt-dev/probe/pkg/prober"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/apis/configmap"
	"github.com/tilt-dev/tilt/internal/controllers/apis/trigger"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/engine/local"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/timecmp"
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
	client        ctrlclient.Client
	st            store.RStore
	clock         clockwork.Clock
	requeuer      *indexer.Requeuer

	mu sync.Mutex
}

var _ store.TearDowner = &Controller{}

func (r *Controller) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&Cmd{}).
		Watches(&source.Kind{Type: &ConfigMap{}},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue)).
		Watches(r.requeuer, handler.Funcs{})

	trigger.SetupControllerStartOn(b, r.indexer, func(obj ctrlclient.Object) *v1alpha1.StartOnSpec {
		return obj.(*v1alpha1.Cmd).Spec.StartOn
	})
	trigger.SetupControllerRestartOn(b, r.indexer, func(obj ctrlclient.Object) *v1alpha1.RestartOnSpec {
		return obj.(*v1alpha1.Cmd).Spec.RestartOn
	})

	return b, nil
}

func NewController(ctx context.Context, execer Execer, proberManager ProberManager, client ctrlclient.Client, st store.RStore, clock clockwork.Clock, scheme *runtime.Scheme) *Controller {
	return &Controller{
		globalCtx:     ctx,
		indexer:       indexer.NewIndexer(scheme, indexCmd),
		clock:         clock,
		execer:        execer,
		procs:         make(map[types.NamespacedName]*currentProcess),
		proberManager: proberManager,
		client:        client,
		st:            st,
		requeuer:      indexer.NewRequeuer(),
	}
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

func inputsFromButton(button *v1alpha1.UIButton) []input {
	if button == nil {
		return nil
	}
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

type triggerEvents struct {
	lastRestartEventTime metav1.MicroTime
	lastRestartButton    *v1alpha1.UIButton
	lastStartEventTime   metav1.MicroTime
	lastStartButton      *v1alpha1.UIButton
}

func (c *Controller) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	name := req.NamespacedName
	cmd := &Cmd{}
	err := c.client.Get(ctx, name, cmd)
	c.indexer.OnReconcile(name, cmd)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, fmt.Errorf("cmd reconcile: %v", err)
	}

	if apierrors.IsNotFound(err) || cmd.ObjectMeta.DeletionTimestamp != nil {
		c.stop(name)
		delete(c.procs, name)
		return ctrl.Result{}, nil
	}

	disableStatus, err := configmap.MaybeNewDisableStatus(ctx, c.client, cmd.Spec.DisableSource, cmd.Status.DisableStatus)
	if err != nil {
		return ctrl.Result{}, err
	}

	proc := c.ensureProc(name)
	proc.mutateStatus(func(status *v1alpha1.CmdStatus) {
		status.DisableStatus = disableStatus
	})

	disabled := disableStatus.State == v1alpha1.DisableStateDisabled
	if disabled {
		// Disabling should both stop the process, and make it look like
		// it didn't previously run.
		c.stop(name)
		proc.spec = v1alpha1.CmdSpec{}
		proc.lastStartOnEventTime = metav1.MicroTime{}
		proc.lastRestartOnEventTime = metav1.MicroTime{}
	}

	if cmd.Annotations[v1alpha1.AnnotationManagedBy] == "local_resource" {
		// Until resource dependencies are expressed in the API,
		// we can't use reconciliation to deploy Cmd objects
		// that are part of local_resource.
		err := c.maybeUpdateObjectStatus(ctx, cmd)
		if err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, nil
	}

	var te triggerEvents
	te.lastRestartEventTime, te.lastRestartButton, _, err = trigger.LastRestartEvent(ctx, c.client, cmd.Spec.RestartOn)
	if err != nil {
		return ctrl.Result{}, err
	}
	te.lastStartEventTime, te.lastStartButton, err = trigger.LastStartEvent(ctx, c.client, cmd.Spec.StartOn)
	if err != nil {
		return ctrl.Result{}, err
	}
	startOn := cmd.Spec.StartOn
	waitsOnStartOn := startOn != nil && len(startOn.UIButtons) > 0

	lastSpec := proc.spec
	lastRestartOnEventTime := proc.lastRestartOnEventTime
	lastStartOnEventTime := proc.lastStartOnEventTime

	restartOnTriggered := !timecmp.BeforeOrEqual(te.lastRestartEventTime, lastRestartOnEventTime)
	startOnTriggered := !timecmp.BeforeOrEqual(te.lastStartEventTime, lastStartOnEventTime)
	execSpecChanged := !cmdExecEqual(lastSpec, cmd.Spec)

	if !disabled {
		// any change to the spec means we should stop the command immediately
		if execSpecChanged {
			c.stop(name)
		}

		if execSpecChanged && waitsOnStartOn && !startOnTriggered {
			// If the cmd spec has changed since the last run,
			// and StartOn hasn't triggered yet, set the status to waiting.
			proc.mutateStatus(func(status *v1alpha1.CmdStatus) {
				status.Waiting = &CmdStateWaiting{
					Reason: waitingOnStartOnReason,
				}
				status.Running = nil
				status.Terminated = nil
				status.Ready = false
			})
		} else if execSpecChanged || restartOnTriggered || startOnTriggered {
			// Otherwise, any change, new start event, or new restart event
			// should restart the process to pick up changes.
			_ = c.runInternal(ctx, cmd, te)
		}
	}

	err = c.maybeUpdateObjectStatus(ctx, cmd)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (c *Controller) maybeUpdateObjectStatus(ctx context.Context, cmd *v1alpha1.Cmd) error {
	newStatus := c.ensureProc(types.NamespacedName{Name: cmd.Name}).copyStatus()
	if apicmp.DeepEqual(newStatus, cmd.Status) {
		return nil
	}

	update := cmd.DeepCopy()
	update.Status = newStatus
	err := c.client.Status().Update(ctx, update)
	if err != nil {
		return err
	}
	c.st.Dispatch(local.NewCmdUpdateStatusAction(update))
	return nil
}

// Forces the command to run now.
//
// This is a hack to get local_resource commands into the API server,
// even though the API server doesn't have a notion of resource deps yet.
//
// Blocks until the command is finished, then returns its status.
func (c *Controller) ForceRun(ctx context.Context, cmd *v1alpha1.Cmd) (*v1alpha1.CmdStatus, error) {
	c.mu.Lock()
	doneCh := c.runInternal(ctx, cmd, triggerEvents{})
	c.mu.Unlock()

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-doneCh:
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	status := c.ensureProc(types.NamespacedName{Name: cmd.Name}).copyStatus()
	return &status, nil
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
	} else if i.status.Hidden != nil {
		return i.status.Hidden.Value
	}
	return ""
}

type input struct {
	spec   v1alpha1.UIInputSpec
	status v1alpha1.UIInputStatus
}

// Ensures there's a current cmd tracker.
func (c *Controller) ensureProc(name types.NamespacedName) *currentProcess {
	proc, ok := c.procs[name]
	if !ok {
		proc = &currentProcess{}
		c.procs[name] = proc
	}
	return proc
}

// Runs the command unconditionally, stopping any currently running command.
//
// The filewatches and buttons are needed for bookkeeping on how the command
// was triggered.
//
// Returns a channel that closes when the Cmd is finished.
func (c *Controller) runInternal(ctx context.Context,
	cmd *v1alpha1.Cmd,
	te triggerEvents) chan struct{} {
	name := types.NamespacedName{Name: cmd.Name}
	c.stop(name)

	proc := c.ensureProc(name)
	proc.spec = cmd.Spec
	proc.isServer = cmd.ObjectMeta.Annotations[local.AnnotationOwnerKind] == "CmdServer"

	proc.lastRestartOnEventTime = te.lastRestartEventTime
	proc.lastStartOnEventTime = te.lastStartEventTime

	var inputs []input
	if !timecmp.BeforeOrEqual(proc.lastRestartOnEventTime, proc.lastStartOnEventTime) {
		inputs = inputsFromButton(te.lastRestartButton)
	} else {
		inputs = inputsFromButton(te.lastStartButton)
	}

	ctx, proc.cancelFunc = context.WithCancel(ctx)
	proc.statusMu.Lock()
	defer proc.statusMu.Unlock()

	status := &(proc.statusInternal)
	status.Running = nil
	status.Waiting = &CmdStateWaiting{}
	status.Terminated = nil
	status.Ready = false

	ctx = store.MustObjectLogHandler(ctx, c.st, cmd)
	spec := cmd.Spec

	if spec.ReadinessProbe != nil {
		probeResultFunc := c.handleProbeResultFunc(ctx, name, proc)
		probeWorker, err := probeWorkerFromSpec(
			c.proberManager,
			spec.ReadinessProbe,
			probeResultFunc)
		if err != nil {
			logger.Get(ctx).Errorf("Invalid readiness probe: %v", err)
			status.Terminated = &CmdStateTerminated{
				ExitCode: 1,
				Reason:   fmt.Sprintf("Invalid readiness probe: %v", err),
			}
			status.Waiting = nil
			status.Running = nil
			status.Ready = false

			proc.doneCh = make(chan struct{})
			close(proc.doneCh)
			return proc.doneCh
		}
		proc.probeWorker = probeWorker
	}

	startedAt := apis.NewMicroTime(c.clock.Now())

	env := append([]string{}, spec.Env...)
	for _, input := range inputs {
		env = append(env, fmt.Sprintf("%s=%s", input.spec.Name, input.stringValue()))
	}

	cmdModel := model.Cmd{
		Argv: spec.Args,
		Dir:  spec.Dir,
		Env:  env,
	}
	statusCh := c.execer.Start(ctx, cmdModel, logger.Get(ctx).Writer(logger.InfoLvl))
	proc.doneCh = make(chan struct{})

	go c.processStatuses(ctx, statusCh, proc, name, startedAt)

	return proc.doneCh
}

func (c *Controller) handleProbeResultFunc(ctx context.Context, name types.NamespacedName, proc *currentProcess) probe.ResultFunc {
	return func(result prober.Result, statusChanged bool, output string, err error) {
		if ctx.Err() != nil {
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

		// TODO(milas): this isn't quite right - we might end up setting
		// 	a terminated process to ready, for example; in practice, we
		// 	should update internal state on any goroutine/async trackers
		// 	and trigger a reconciliation, which can then evaluate the full
		// 	state + current spec
		proc.statusMu.Lock()
		defer proc.statusMu.Unlock()

		status := &(proc.statusInternal)
		if status.Ready != ready {
			status.Ready = ready
			c.requeuer.Add(name)
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

const waitingOnStartOnReason = "cmd StartOn has not been triggered"

func (c *Controller) processStatuses(
	ctx context.Context,
	statusCh chan statusAndMetadata,
	proc *currentProcess,
	name types.NamespacedName,
	startedAt metav1.MicroTime) {
	defer close(proc.doneCh)

	var initProbeWorker sync.Once

	for sm := range statusCh {
		if sm.status == Unknown {
			continue
		}

		if sm.status == Error || sm.status == Done {
			// This is a hack until CmdServer is a real object.
			if proc.isServer && sm.exitCode == 0 {
				logger.Get(ctx).Errorf("Server exited with exit code 0")
			}

			proc.mutateStatus(func(status *v1alpha1.CmdStatus) {
				status.Waiting = nil
				status.Running = nil
				status.Terminated = &CmdStateTerminated{
					PID:        int32(sm.pid),
					Reason:     sm.reason,
					ExitCode:   int32(sm.exitCode),
					StartedAt:  startedAt,
					FinishedAt: apis.NewMicroTime(c.clock.Now()),
				}
			})
			c.requeuer.Add(name)
		} else if sm.status == Running {
			if proc.probeWorker != nil {
				initProbeWorker.Do(func() {
					go proc.probeWorker.Run(ctx)
				})
			}

			proc.mutateStatus(func(status *v1alpha1.CmdStatus) {
				status.Waiting = nil
				status.Terminated = nil
				status.Running = &CmdStateRunning{
					PID:       int32(sm.pid),
					StartedAt: startedAt,
				}

				if proc.probeWorker == nil {
					status.Ready = true
				}
			})
			c.requeuer.Add(name)
		}
	}
}

// Find all the objects we need to watch based on the Cmd model.
func indexCmd(obj client.Object) []indexer.Key {
	cmd := obj.(*v1alpha1.Cmd)
	result := []indexer.Key{}
	if cmd.Spec.DisableSource != nil {
		cm := cmd.Spec.DisableSource.ConfigMap
		if cm != nil {
			gvk := v1alpha1.SchemeGroupVersion.WithKind("ConfigMap")
			result = append(result, indexer.Key{
				Name: types.NamespacedName{Name: cm.Name},
				GVK:  gvk,
			})
		}
	}
	return result
}

// currentProcess represents the current process for a Manifest, so that Controller can
// make sure there's at most one process per Manifest.
// (note: it may not be running yet, or may have already finished)
type currentProcess struct {
	spec       CmdSpec
	cancelFunc context.CancelFunc
	// closed when the process finishes executing, intentionally or not
	doneCh      chan struct{}
	probeWorker *probe.Worker
	isServer    bool

	lastRestartOnEventTime metav1.MicroTime
	lastStartOnEventTime   metav1.MicroTime

	// We have a lock that ONLY protects the status.
	statusMu       sync.Mutex
	statusInternal v1alpha1.CmdStatus
}

func (p *currentProcess) copyStatus() v1alpha1.CmdStatus {
	p.statusMu.Lock()
	defer p.statusMu.Unlock()
	return *(p.statusInternal.DeepCopy())
}

func (p *currentProcess) mutateStatus(update func(status *v1alpha1.CmdStatus)) {
	p.statusMu.Lock()
	defer p.statusMu.Unlock()
	update(&p.statusInternal)
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
