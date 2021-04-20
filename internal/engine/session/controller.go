package session

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"

	"github.com/tilt-dev/tilt/pkg/logger"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/store"
	session "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// Controller summarizes engine state of resources for the active Tilt session (i.e. invocation of up/ci).
//
// Part of the Session spec includes an exit condition, which is evaluated here and reflected on the Session status.
// The engine will react to changes in the status and exit once Done is true, propagating the error if one exists.
//
// While using an apiserver type and updating the corresponding entity in the apiserver itself, this is not currently
// a reconciler due to heavy dependence on engine internals. It's very likely this will look very different once it
// has been converted to a reconciler. (Ideally, there will also be much less special case conversion logic as the data
// models on which this controller depends evolve during migration to apiserver.)
type Controller struct {
	pid       int64
	startTime time.Time
	client    ctrlclient.Client

	// The last status object sent to the server.
	lastStatus *session.SessionStatus

	// The last session object returned by the server.
	// Note that the server may annotate and transform this
	// on top of what we sent.
	session *session.Session
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

	if c.session == nil {
		if initialized, err := c.initialize(ctx, st); err != nil {
			st.Dispatch(store.NewErrorAction(fmt.Errorf("failed to initialize Session controller: %v", err)))
			return
		} else if !initialized {
			// engine is still starting up, no-op until ready for initialization
			return
		}
	}

	newStatus := c.makeLatestStatus(st)
	if err := c.handleLatestStatus(ctx, st, newStatus); err != nil {
		logger.Get(ctx).Debugf("failed to update Session status: %v", err)
	}
}

func (c *Controller) initialize(ctx context.Context, st store.RStore) (bool, error) {
	s := c.makeSession(st)
	if s == nil {
		return false, nil
	}

	// TODO(milas): rather than implicitly creating the Session object here, it should
	// 	be created explicitly as part of loading the Tiltfile
	if err := c.client.Create(ctx, s); err != nil {
		return false, fmt.Errorf("failed to create Session API object: %v", err)
	}

	c.session = s

	return true, nil
}

func (c *Controller) makeSession(st store.RStore) *session.Session {
	state := st.RLockState()
	defer st.RUnlockState()

	// engine hasn't finished initialization - Tiltfile hasn't been loaded yet
	if state.TiltfilePath == "" {
		return nil
	}

	s := &session.Session{
		ObjectMeta: metav1.ObjectMeta{
			Name: "Tiltfile",
		},
		Spec: session.SessionSpec{
			TiltfilePath: state.TiltfilePath,
		},
		Status: session.SessionStatus{
			PID:       c.pid,
			StartTime: metav1.NewMicroTime(c.startTime),
		},
	}

	// currently, manual + CI are the only supported modes; the apiserver will validate this field and reject
	// the object on creation if it doesn't conform, so there's no additional validation/error-handling here
	switch state.EngineMode {
	case store.EngineModeUp:
		s.Spec.ExitCondition = session.ExitConditionManual
	case store.EngineModeCI:
		s.Spec.ExitCondition = session.ExitConditionCI
	}

	return s
}

func (c *Controller) makeLatestStatus(st store.RStore) *session.SessionStatus {
	state := st.RLockState()
	defer st.RUnlockState()

	status := &session.SessionStatus{
		PID:       c.pid,
		StartTime: metav1.NewMicroTime(c.startTime),
	}

	status.Targets = append(status.Targets, tiltfileTarget(state))

	// determine the reason any resources (and thus all of their targets) are waiting (aka "holds")
	// N.B. we don't actually care about what's "next" to build, but the info comes alongside that
	_, holds := buildcontrol.NextTargetToBuild(state)

	for _, mt := range state.ManifestTargets {
		status.Targets = append(status.Targets, targetsForResource(mt, holds)...)
	}
	// ensure consistent ordering to avoid unnecessary updates
	sort.SliceStable(status.Targets, func(i, j int) bool {
		return status.Targets[i].Name < status.Targets[j].Name
	})

	processExitCondition(c.session.Spec.ExitCondition, status)
	return status
}

func (c *Controller) handleLatestStatus(ctx context.Context, st store.RStore, newStatus *session.SessionStatus) error {
	// Use the lastStatus to check for changes, so we don't have to worry
	// about server-side changes affecting the equality check.
	if equality.Semantic.DeepEqual(c.lastStatus, newStatus) {
		return nil
	}
	c.lastStatus = newStatus.DeepCopy()

	// deep copy is made to avoid tainting local version on failure
	updated := c.session.DeepCopy()
	updated.Status = *newStatus

	if err := c.client.Status().Update(ctx, updated); err != nil {
		return err
	}

	c.session = updated
	st.Dispatch(NewSessionUpdateStatusAction(updated))

	return nil
}

func processExitCondition(exitCondition session.ExitCondition, status *session.SessionStatus) {
	if exitCondition == session.ExitConditionManual {
		return
	} else if exitCondition != session.ExitConditionCI {
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
		} else if res.State.Active != nil && (!res.State.Active.Ready || res.Type == session.TargetTypeJob) {
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
