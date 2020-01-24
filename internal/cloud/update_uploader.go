package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/windmilleng/tilt/internal/cloud/cloudurl"
	"github.com/windmilleng/tilt/internal/feature"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/token"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

type UpdateUploader struct {
	client HttpClient
	addr   cloudurl.Address

	lastCompletedBuildCount int
	lastFinishTime          time.Time
}

func NewUpdateUploader(client HttpClient, addr cloudurl.Address) *UpdateUploader {
	return &UpdateUploader{
		client: client,
		addr:   addr,
	}
}

// TODO(nick): Generate these with protobufs.
type updateServiceSpec struct {
	Name string `json:"name"`
}

type snapshotID struct {
	ID string `json:"id"`
}

type update struct {
	Service      updateServiceSpec `json:"service"`
	StartTime    string            `json:"start_time"`
	Duration     string            `json:"duration"`
	IsLiveUpdate bool              `json:"is_live_update"`

	// 0 = SUCCESS, 1 = FAILURE
	Result            int    `json:"result"`
	ResultDescription string `json:"result_description"`

	SnapshotID snapshotID `json:"snapshot_id"`
}

type teamID struct {
	ID string `json:"id"`
}

type updatePayload struct {
	TeamID  teamID   `json:"team_id"`
	Updates []update `json:"updates"`
}

func (p updatePayload) empty() bool {
	return len(p.Updates) == 0
}

type updateTask struct {
	token         token.Token
	updatePayload updatePayload
}

func (t updateTask) empty() bool {
	return t.updatePayload.empty()
}

func (t updateTask) updates() []update {
	return t.updatePayload.Updates
}

func (u *UpdateUploader) putUpdatesURL() string {
	url := cloudurl.URL(string(u.addr))
	url.Path = "/api/usage/team/put_updates"
	return url.String()
}

// Check the engine state to see if we have any updates.
func (u *UpdateUploader) makeUpdates(ctx context.Context, st store.RStore) updateTask {
	state := st.RLockState()
	defer st.RUnlockState()

	// Check if this feature is disabled
	if !state.Features[feature.UpdateHistory] {
		return updateTask{}
	}

	// If we don't have an authenticated token or team-name,
	// we won't be able to upload anything anyway.
	if state.Token == "" || state.TeamName == "" || state.TiltCloudUsername == "" {
		return updateTask{}
	}

	// Do a quick check to see if any builds have completed since we last checked.
	if state.CompletedBuildCount == 0 || state.CompletedBuildCount <= u.lastCompletedBuildCount {
		return updateTask{}
	}

	// OK, we know we have work to do!

	highWaterMark := u.lastFinishTime
	updates := []update{}

	processManifestState := func(name model.ManifestName, status store.ManifestState) {
		for _, record := range status.BuildHistory {
			// The BuildHistory is stored most-recent first, so we can stop iterating
			// as soon as we see one newer than the high-water mark.
			if !record.FinishTime.After(u.lastFinishTime) {
				break
			}

			if record.FinishTime.After(highWaterMark) {
				highWaterMark = record.FinishTime
			}

			resultCode := 0
			resultDescription := ""
			if record.Error != nil {
				resultCode = 1
				resultDescription = record.Error.Error()
			}

			updates = append(updates, update{
				Service: updateServiceSpec{
					Name: name.String(),
				},
				StartTime:         record.StartTime.Format(time.RFC3339),
				Duration:          record.Duration().String(),
				IsLiveUpdate:      record.HasBuildType(model.BuildTypeLiveUpdate),
				Result:            resultCode,
				ResultDescription: resultDescription,
			})
		}
	}

	processManifestState(store.TiltfileManifestName, state.TiltfileState)
	for _, target := range state.ManifestTargets {
		processManifestState(target.Manifest.Name, *(target.State))
	}

	u.lastFinishTime = highWaterMark
	u.lastCompletedBuildCount = state.CompletedBuildCount

	return updateTask{
		token: state.Token,
		updatePayload: updatePayload{
			TeamID:  teamID{ID: state.TeamName},
			Updates: updates,
		},
	}
}

func (u *UpdateUploader) sendUpdates(ctx context.Context, task updateTask) {
	buf := bytes.NewBuffer(nil)
	encoder := json.NewEncoder(buf)
	err := encoder.Encode(task.updatePayload)
	if err != nil {
		logger.Get(ctx).Infof("Error encoding updates: %v", err)
		return
	}

	request, err := http.NewRequest(http.MethodPost, u.putUpdatesURL(), buf)
	if err != nil {
		logger.Get(ctx).Infof("Error sending updates: %v", err)
		return
	}

	request.Header.Set(TiltTokenHeaderName, task.token.String())
	response, err := u.client.Do(request)
	if err != nil {
		logger.Get(ctx).Infof("Error sending updates: %v", err)
		return
	}
	if response.StatusCode != http.StatusOK {
		b, err := ioutil.ReadAll(response.Body)
		if err != nil {
			logger.Get(ctx).Infof("Error reading update-send response body: %v", err)
			return
		}

		logger.Get(ctx).Infof("Error sending updates. status: %s. response: %s", response.Status, b)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	logger.Get(ctx).Infof("Update records reported to %s", u.addr)
}

func (u *UpdateUploader) OnChange(ctx context.Context, st store.RStore) {
	task := u.makeUpdates(ctx, st)
	if !task.empty() {
		u.sendUpdates(ctx, task)
	}
}

var _ store.Subscriber = &UpdateUploader{}
