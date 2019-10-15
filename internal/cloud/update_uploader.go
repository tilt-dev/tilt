package cloud

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/duration"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/windmilleng/tilt/internal/feature"
	"github.com/windmilleng/tilt/internal/store"
)

type UpdateUploader struct {
	client HttpClient
	addr   Address

	lastCompletedBuildCount int
	lastFinishTime          time.Time
}

func NewUpdateUploader(client HttpClient, addr Address) *UpdateUploader {
	return &UpdateUploader{
		client: client,
		addr:   addr,
	}
}

// TODO(nick): Generate these with protobufs.
type updateServiceSpec struct {
	Name string `json:"name"`
}

type update struct {
	Service      updateServiceSpec    `json:"service"`
	StartTime    *timestamp.Timestamp `json:"start_time"`
	Duration     *duration.Duration   `json:"duration"`
	IsLiveUpdate bool                 `json:"is_live_update"`

	// 0 = SUCCESS, 1 = FAILURE
	Result            int    `json:"result"`
	ResultDescription string `json:"result_description"`

	// TODO(nick): auto-create Snapshot IDs?
}

// Check the engine state to see if we have any updates.
func (u *UpdateUploader) makeUpdates(st store.RStore) []update {
	state := st.RLockState()
	defer st.RUnlockState()

	// Check if this feature is disabled
	if !state.Features[feature.UpdateHistory] {
		return nil
	}

	// Do a quick check to see if any builds have completed since we last checked.
	if state.CompletedBuildCount == 0 || state.CompletedBuildCount <= u.lastCompletedBuildCount {
		return nil
	}

	// OK, we know we have work to do!

	highWaterMark := u.lastFinishTime
	result := []update{}
	for _, target := range state.ManifestTargets {
		manifest := target.Manifest
		status := target.State

		for _, record := range status.BuildHistory {
			// The BuildHistory is stored most-recent first, so we can stop iterating
			// as soon as we see one newer than the high-water mark.
			if record.FinishTime.Before(u.lastFinishTime) {
				break
			}

			if record.FinishTime.After(highWaterMark) {
				highWaterMark = record.FinishTime
			}

			startTime, err := ptypes.TimestampProto(record.StartTime)
			if err != nil {
				// Just silently ignore errors.
				continue
			}

			resultCode := 0
			resultDescription := ""
			if record.Error != nil {
				resultCode = 1
				resultDescription = record.Error.Error()
			}

			result = append(result, update{
				Service: updateServiceSpec{
					Name: manifest.Name.String(),
				},
				StartTime: startTime,
				Duration:  ptypes.DurationProto(record.Duration()),

				// TODO(nick): Fill in is_live_update

				Result:            resultCode,
				ResultDescription: resultDescription,
			})
		}

	}

	u.lastFinishTime = highWaterMark
	u.lastCompletedBuildCount = state.CompletedBuildCount

	return result
}

func (u *UpdateUploader) sendUpdates(ctx context.Context, updates []update) {

}

func (u *UpdateUploader) OnChange(ctx context.Context, st store.RStore) {
	updates := u.makeUpdates(st)
	if len(updates) > 0 {
		u.sendUpdates(ctx, updates)
	}
}

var _ store.Subscriber = &UpdateUploader{}
