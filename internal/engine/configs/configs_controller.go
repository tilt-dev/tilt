package configs

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	ctrltiltfile "github.com/tilt-dev/tilt/internal/controllers/core/tiltfile"
	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

type ConfigsController struct {
	ctrlClient       ctrlclient.Client
	buildSource      *ctrltiltfile.BuildSource
	loadStartedCount int // used to synchronize with state
}

func NewConfigsController(ctrlClient ctrlclient.Client, buildSource *ctrltiltfile.BuildSource) *ConfigsController {
	return &ConfigsController{
		ctrlClient:  ctrlClient,
		buildSource: buildSource,
	}
}

// Modeled after BuildController.needsBuild and NextBuildReason(). Check to see that:
// 1) There's currently no Tiltfile build running,
// 2) There are pending file changes, and
// 3) Those files have changed since the last Tiltfile build
//    (so that we don't keep re-running a failed build)
// 4) OR the command-line args have changed since the last Tiltfile build
// 5) OR user has manually triggered a Tiltfile build
func (cc *ConfigsController) needsBuild(ctx context.Context, st store.RStore) (*ctrltiltfile.BuildEntry, bool) {
	state := st.RLockState()
	defer st.RUnlockState()

	// Don't start the next build until the previous action has been recorded,
	// so that we don't accidentally repeat the same build.
	if cc.loadStartedCount != state.StartedTiltfileLoadCount {
		return nil, false
	}

	// Don't start the next build if the last completion hasn't been recorded yet.
	for _, ms := range state.TiltfileStates {
		isRunning := !ms.CurrentBuild.StartTime.IsZero()
		if isRunning {
			return nil, false
		}
	}

	for _, name := range state.TiltfileDefinitionOrder {
		tfState, ok := state.TiltfileStates[name]
		if !ok {
			continue
		}

		var reason model.BuildReason
		lastStartTime := tfState.LastBuild().StartTime
		if !tfState.StartedFirstBuild() {
			reason = reason.With(model.BuildReasonFlagInit)
		}

		hasPendingChanges, _ := tfState.HasPendingChanges()
		if hasPendingChanges {
			reason = reason.With(model.BuildReasonFlagChangedFiles)
		}

		if state.UserConfigState.ArgsChangeTime.After(lastStartTime) {
			reason = reason.With(model.BuildReasonFlagTiltfileArgs)
		}

		if state.ManifestInTriggerQueue(name) {
			reason = reason.With(tfState.TriggerReason)
		}

		if reason == model.BuildReasonNone {
			continue
		}

		filesChanged := []string{}
		for _, st := range tfState.BuildStatuses {
			for k := range st.PendingFileChanges {
				filesChanged = append(filesChanged, k)
			}
		}
		filesChanged = sliceutils.DedupedAndSorted(filesChanged)

		tiltfilePath, err := state.RelativeTiltfilePath()
		if err != nil {
			st.Dispatch(store.NewErrorAction(err))
		}

		return &ctrltiltfile.BuildEntry{
			Name:                  name,
			FilesChanged:          filesChanged,
			BuildReason:           reason,
			UserConfigState:       state.UserConfigState,
			TiltfilePath:          tiltfilePath,
			CheckpointAtExecStart: state.LogStore.Checkpoint(),
			EngineMode:            state.EngineMode,
		}, true
	}

	return nil, false
}

func (cc *ConfigsController) OnChange(ctx context.Context, st store.RStore, _ store.ChangeSummary) error {
	entry, ok := cc.needsBuild(ctx, st)
	if !ok {
		return nil
	}

	newTF := &v1alpha1.Tiltfile{
		ObjectMeta: metav1.ObjectMeta{Name: entry.Name.String()},
		Spec: v1alpha1.TiltfileSpec{
			Path: entry.TiltfilePath,
		},
	}
	var tf v1alpha1.Tiltfile
	err := cc.ctrlClient.Get(ctx, types.NamespacedName{Name: entry.Name.String()}, &tf)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if apierrors.IsNotFound(err) {
		err := cc.ctrlClient.Create(ctx, newTF)
		if err != nil {
			return err
		}
	} else if !apicmp.DeepEqual(tf.Spec, newTF.Spec) {
		update := tf.DeepCopy()
		update.Spec = newTF.Spec
		err := cc.ctrlClient.Update(ctx, update)
		if err != nil {
			return err
		}
	}

	// Don't increment until we know we've updated the apiserver.
	cc.loadStartedCount++
	entry.LoadCount = cc.loadStartedCount
	cc.buildSource.SetEntry(entry)
	return nil
}
