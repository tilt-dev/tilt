package buildcontrol

import (
	"time"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
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
	FinishTime   time.Time
	Error        error
}

func (BuildCompleteAction) Action() {}

func NewBuildCompleteAction(mn model.ManifestName, spanID logstore.SpanID, result store.BuildResultSet, err error) BuildCompleteAction {
	return BuildCompleteAction{
		ManifestName: mn,
		SpanID:       spanID,
		Result:       result,
		FinishTime:   time.Now(),
		Error:        err,
	}
}
