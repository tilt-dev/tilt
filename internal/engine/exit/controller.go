package exit

import (
	"context"
	"fmt"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"

	"github.com/tilt-dev/tilt/pkg/logger"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/store"
	tiltrun "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// Controller handles normal process termination. Either Tilt completed all its work,
// or it determined that it was unable to complete the work it was assigned.
type Controller struct {
	pid       int64
	startTime time.Time
	client    ctrlclient.Client

	mu      sync.Mutex
	tiltRun *tiltrun.TiltRun
}

var _ store.Subscriber = &Controller{}

func NewController(cli ctrlclient.Client) *Controller {
	return &Controller{
		pid:       int64(os.Getpid()),
		startTime: time.Now(),
		client:    cli,
	}
}

func (c *Controller) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) {
	if summary.IsLogOnly() {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.tiltRun == nil {
		if initialized, err := c.initialize(ctx, st); err != nil {
			st.Dispatch(store.NewErrorAction(fmt.Errorf("failed to initialize ExitController: %v", err)))
			return
		} else if !initialized {
			// engine is still starting up, no-op until ready for initialization
			return
		}
	}

	newStatus := c.makeLatestStatus(st)
	if err := c.handleLatestStatus(ctx, st, newStatus); err != nil {
		logger.Get(ctx).Debugf("failed to update TiltRun status: %v", err)
	}
}

func (c *Controller) initialize(ctx context.Context, st store.RStore) (bool, error) {
	tiltRun := c.makeTiltRun(st)
	if tiltRun == nil {
		return false, nil
	}

	// TODO(milas): rather than implicitly creating the TiltRun object here, it should
	// 	be created explicitly as part of loading the Tiltfile
	if err := c.client.Create(ctx, tiltRun); err != nil {
		return false, fmt.Errorf("failed to create TiltRun API object: %v", err)
	}

	c.tiltRun = tiltRun

	return true, nil
}

func (c *Controller) makeTiltRun(st store.RStore) *tiltrun.TiltRun {
	state := st.RLockState()
	defer st.RUnlockState()

	// engine hasn't finished initialization - Tiltfile hasn't been loaded yet
	if state.TiltfilePath == "" {
		return nil
	}

	tiltRun := &tiltrun.TiltRun{
		ObjectMeta: metav1.ObjectMeta{
			Name: "Tiltfile",
		},
		Spec: tiltrun.TiltRunSpec{
			TiltfilePath: state.TiltfilePath,
		},
		Status: tiltrun.TiltRunStatus{
			PID:       c.pid,
			StartTime: metav1.NewMicroTime(c.startTime),
		},
	}

	// currently, manual + CI are the only supported modes; the apiserver will validate this field and reject
	// the object on creation if it doesn't conform, so there's no additional validation/error-handling here
	switch state.EngineMode {
	case store.EngineModeUp:
		tiltRun.Spec.ExitCondition = tiltrun.ExitConditionManual
	case store.EngineModeCI:
		tiltRun.Spec.ExitCondition = tiltrun.ExitConditionCI
	}

	return tiltRun
}

func (c *Controller) makeLatestStatus(st store.RStore) *tiltrun.TiltRunStatus {
	state := st.RLockState()
	defer st.RUnlockState()

	status := &tiltrun.TiltRunStatus{
		PID:       c.pid,
		StartTime: metav1.NewMicroTime(c.startTime),
	}

	tiltfileResource := tiltfileTarget(state.TiltfileState)

	_, holds := buildcontrol.NextTargetToBuild(state)

	var targetResources []tiltrun.Target
	for _, mt := range state.ManifestTargets {
		targetResources = append(targetResources, targetsForResource(mt, holds)...)
	}
	// ensure consistent ordering to avoid unnecessary updates
	sort.SliceStable(targetResources, func(i, j int) bool {
		return targetResources[i].Name < targetResources[j].Name
	})

	status.Targets = append([]tiltrun.Target{tiltfileResource}, targetResources...)

	processExitCondition(c.tiltRun.Spec.ExitCondition, status)
	return status
}

func (c *Controller) handleLatestStatus(ctx context.Context, st store.RStore, newStatus *tiltrun.TiltRunStatus) error {
	if equality.Semantic.DeepEqual(&c.tiltRun.Status, newStatus) {
		return nil
	}

	// deep copy is made to avoid tainting local version on failure
	updated := c.tiltRun.DeepCopy()
	updated.Status = *newStatus

	if err := c.client.Status().Update(ctx, updated); err != nil {
		return err
	}

	c.tiltRun = updated
	st.Dispatch(NewTiltRunUpdateStatusAction(updated))

	return nil
}

func processExitCondition(exitCondition tiltrun.ExitCondition, status *tiltrun.TiltRunStatus) {
	if exitCondition == tiltrun.ExitConditionManual {
		return
	} else if exitCondition != tiltrun.ExitConditionCI {
		status.Done = true
		status.Error = fmt.Sprintf("unsupported exit condition: %s", exitCondition)
	}

	allResourcesOK := true
	for _, res := range status.Targets {
		if res.State.Waiting == nil && res.State.Active == nil && res.State.Terminated == nil {
			// if all states are nil, the target has not been requested to run, e.g. auto_init=False
			continue
		}
		if res.State.Terminated != nil && res.State.Terminated.Error != "" {
			status.Done = true
			status.Error = res.State.Terminated.Error
			return
		}
		if res.State.Waiting != nil {
			allResourcesOK = false
		} else if res.State.Active != nil && (!res.State.Active.Ready || res.Type == tiltrun.TargetTypeJob) {
			// jobs must run to completion
			allResourcesOK = false
		}
	}

	// Tiltfile is _always_ a target, so ensure that there's at least one other real target, or it's possible to
	// exit before the targets have actually been initialized
	if allResourcesOK && len(status.Targets) > 1 {
		status.Done = true
	}
}

// errToString returns a stringified version of an error or an empty string if the error is nil.
func errToString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
