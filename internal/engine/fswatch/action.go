package fswatch

import (
	"context"
	"fmt"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/store"
	filewatches "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type FileWatchCreateAction struct {
	FileWatch *filewatches.FileWatch
}

func (a FileWatchCreateAction) Summarize(summary *store.ChangeSummary) {
	if summary.FileWatchSpecs == nil {
		summary.FileWatchSpecs = make(map[types.NamespacedName]bool)
	}
	key := types.NamespacedName{Namespace: a.FileWatch.GetNamespace(), Name: a.FileWatch.GetName()}
	summary.FileWatchSpecs[key] = true
}

func (FileWatchCreateAction) Action() {}

func NewFileWatchCreateAction(fw *filewatches.FileWatch) FileWatchCreateAction {
	return FileWatchCreateAction{FileWatch: fw.DeepCopy()}
}

type FileWatchUpdateAction struct {
	FileWatch *filewatches.FileWatch
}

func (a FileWatchUpdateAction) Summarize(summary *store.ChangeSummary) {
	if summary.FileWatchSpecs == nil {
		summary.FileWatchSpecs = make(map[types.NamespacedName]bool)
	}
	key := types.NamespacedName{Namespace: a.FileWatch.GetNamespace(), Name: a.FileWatch.GetName()}
	summary.FileWatchSpecs[key] = true
}

func (FileWatchUpdateAction) Action() {}

func NewFileWatchUpdateAction(fw *filewatches.FileWatch) FileWatchUpdateAction {
	return FileWatchUpdateAction{FileWatch: fw.DeepCopy()}
}

type FileWatchUpdateStatusAction struct {
	ObjectMeta *metav1.ObjectMeta
	Status     *filewatches.FileWatchStatus
}

func (a FileWatchUpdateStatusAction) Summarize(_ *store.ChangeSummary) {
	// do nothing - we only care about _spec_ changes on the summary
}

func (FileWatchUpdateStatusAction) Action() {}

func NewFileWatchUpdateStatusAction(fw *filewatches.FileWatch) FileWatchUpdateStatusAction {
	return FileWatchUpdateStatusAction{ObjectMeta: fw.GetObjectMeta().DeepCopy(), Status: fw.Status.DeepCopy()}
}

type FileWatchDeleteAction struct {
	Name types.NamespacedName
}

func (a FileWatchDeleteAction) Summarize(summary *store.ChangeSummary) {
	if summary.FileWatchSpecs == nil {
		summary.FileWatchSpecs = make(map[types.NamespacedName]bool)
	}
	summary.FileWatchSpecs[a.Name] = true
}

func (FileWatchDeleteAction) Action() {}

func NewFileWatchDeleteAction(name types.NamespacedName) FileWatchDeleteAction {
	return FileWatchDeleteAction{Name: name}
}

func HandleFileWatchCreateEvent(_ context.Context, state *store.EngineState, action FileWatchCreateAction) {
	name := types.NamespacedName{Namespace: action.FileWatch.GetNamespace(), Name: action.FileWatch.GetName()}
	state.FileWatches[name] = action.FileWatch
}

func HandleFileWatchUpdateEvent(ctx context.Context, state *store.EngineState, action FileWatchUpdateAction) {
	name := types.NamespacedName{Namespace: action.FileWatch.GetNamespace(), Name: action.FileWatch.GetName()}
	fw := state.FileWatches[name]
	if fw == nil {
		return
	}
	action.FileWatch.DeepCopyInto(fw)
	processFileWatchStatus(ctx, state, fw)
}

func HandleFileWatchUpdateStatusEvent(ctx context.Context, state *store.EngineState, action FileWatchUpdateStatusAction) {
	key := types.NamespacedName{Namespace: action.ObjectMeta.GetNamespace(), Name: action.ObjectMeta.GetName()}
	fw := state.FileWatches[key]
	if fw == nil {
		return
	}
	action.ObjectMeta.DeepCopyInto(&fw.ObjectMeta)
	action.Status.DeepCopyInto(&fw.Status)
	processFileWatchStatus(ctx, state, fw)
}

func HandleFileWatchDeleteEvent(_ context.Context, state *store.EngineState, action FileWatchDeleteAction) {
	delete(state.FileWatches, action.Name)
}

func processFileWatchStatus(ctx context.Context, state *store.EngineState, fw *filewatches.FileWatch) {
	status := fw.Status
	if status.Error != "" || len(status.FileEvents) == 0 {
		return
	}

	// since the store is called on EVERY update, can always just look at the last event
	latestEvent := status.FileEvents[len(status.FileEvents)-1]

	targetID, err := targetID(fw)
	if err != nil {
		logger.Get(ctx).Debugf("Failed to get targetID for FileWatch %q to process update: %v", fw.GetName(), err)
		return
	} else if targetID.Empty() {
		return
	}

	// NOTE(nick): BuildController uses these timestamps to determine which files
	// to clear after a build. In particular, it:
	//
	// 1) Grabs the pending files
	// 2) Runs a live update
	// 3) Clears the pending files with timestamps before the live update started.
	//
	// Here's the race condition: suppose a file changes, but it doesn't get into
	// the EngineState until after step (2). That means step (3) will clear the file
	// even though it wasn't live-updated properly. Because as far as we can tell,
	// the file must have been in the EngineState before the build started.
	//
	// Ideally, BuildController should be do more bookkeeping to keep track of
	// which files it consumed from which FileWatches. But we're changing
	// this architecture anyway. For now, we record the time it got into
	// the EngineState, rather than the time it was originally changed.
	now := time.Now()

	if targetID.Type == model.TargetTypeConfigs {
		for _, f := range latestEvent.SeenFiles {
			state.PendingConfigFileChanges[f] = now
		}
		return
	}

	mns := state.ManifestNamesForTargetID(targetID)
	for _, mn := range mns {
		ms, ok := state.ManifestState(mn)
		if !ok {
			return
		}

		status := ms.MutableBuildStatus(targetID)
		for _, f := range latestEvent.SeenFiles {
			status.PendingFileChanges[f] = now
		}
	}
}

func targetID(obj runtime.Object) (model.TargetID, error) {
	metaObj, err := meta.Accessor(obj)
	if err != nil {
		return model.TargetID{}, err
	}
	labelVal := metaObj.GetAnnotations()[filewatches.AnnotationTargetID]
	if labelVal == "" {
		return model.TargetID{}, nil
	}
	targetParts := strings.SplitN(labelVal, ":", 2)
	if len(targetParts) != 2 || targetParts[0] == "" || targetParts[1] == "" {
		return model.TargetID{}, fmt.Errorf("invalid target ID: %q", labelVal)
	}
	return model.TargetID{Type: model.TargetType(targetParts[0]), Name: model.TargetName(targetParts[1])}, nil
}
