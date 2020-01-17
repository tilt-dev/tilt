package buildcontrol

import (
	"time"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"
	"github.com/windmilleng/tilt/pkg/model/logstore"
)

type BuildStartedAction struct {
	ManifestName model.ManifestName
	StartTime    time.Time
	FilesChanged []string
	Reason       model.BuildReason
	SpanID       logstore.SpanID
}

func (BuildStartedAction) Action() {}

type BuildCompleteAction struct {
	ManifestName model.ManifestName
	SpanID       logstore.SpanID
	Result       store.BuildResultSet
	Error        error
}

func (BuildCompleteAction) Action() {}

func NewBuildCompleteAction(mn model.ManifestName, spanID logstore.SpanID, result store.BuildResultSet, err error) BuildCompleteAction {
	return BuildCompleteAction{
		ManifestName: mn,
		SpanID:       spanID,
		Result:       result,
		Error:        err,
	}
}
