package filewatch

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	filewatches "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

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

func HandleFileWatchUpdateStatusEvent(ctx context.Context, state *store.EngineState, action FileWatchUpdateStatusAction) {
	processFileWatchStatus(ctx, state, action.ObjectMeta, action.Status)
}

func processFileWatchStatus(ctx context.Context, state *store.EngineState, meta *metav1.ObjectMeta, status *v1alpha1.FileWatchStatus) {
	if status.Error != "" || len(status.FileEvents) == 0 {
		return
	}

	// since the store is called on EVERY update, can always just look at the last event
	latestEvent := status.FileEvents[len(status.FileEvents)-1]

	targetID, err := targetID(meta)
	if err != nil {
		logger.Get(ctx).Debugf("Failed to get targetID for FileWatch %q to process update: %v", meta.GetName(), err)
		return
	} else if targetID.Empty() {
		return
	}

	mns := state.ManifestNamesForTargetID(targetID)
	for _, mn := range mns {
		ms, ok := state.ManifestState(mn)
		if !ok {
			return
		}

		for _, f := range latestEvent.SeenFiles {
			ms.AddPendingFileChange(targetID, f, latestEvent.Time.Time)
		}
	}
}

func targetID(metaObj *metav1.ObjectMeta) (model.TargetID, error) {
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
