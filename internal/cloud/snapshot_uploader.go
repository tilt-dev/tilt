package cloud

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/feature"
	"github.com/windmilleng/tilt/internal/hud/webview"
	"github.com/windmilleng/tilt/internal/network"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/token"
	"github.com/windmilleng/tilt/pkg/logger"
)

type SnapshotID string

// Auto-uploads a snapshot every time a build completes.
type SnapshotUploader struct {
	client                  HttpClient
	addr                    Address
	lastCompletedBuildCount int
}

func NewSnapshotUploader(client HttpClient, addr Address) *SnapshotUploader {
	return &SnapshotUploader{
		client: client,
		addr:   addr,
	}
}

func (s *SnapshotUploader) newSnapshotURL() string {
	u := network.HostToURL(string(s.addr))
	u.Path = "/api/snapshot/new"
	return u.String()
}

func (s *SnapshotUploader) IDToSnapshotURL(id SnapshotID) string {
	u := network.HostToURL(string(s.addr))
	u.Path = fmt.Sprintf("snapshot/%s", id)
	return u.String()
}

type snapshotIDResponse struct {
	ID string
}

func (s *SnapshotUploader) Upload(token token.Token, teamID string, reader io.Reader) (SnapshotID, error) {
	request, err := http.NewRequest(http.MethodPost, s.newSnapshotURL(), reader)
	if err != nil {
		return "", errors.Wrap(err, "Upload NewRequest")
	}

	request.Header.Set(TiltTokenHeaderName, token.String())
	if teamID != "" {
		request.Header.Set(TiltTeamIDNameHeaderName, teamID)
	}

	response, err := s.client.Do(request)
	if err != nil {
		return "", errors.Wrap(err, "Upload")
	}
	defer func() {
		_ = response.Body.Close()
	}()

	// unpack response with snapshot ID
	var resp snapshotIDResponse
	decoder := json.NewDecoder(response.Body)
	err = decoder.Decode(&resp)
	if err != nil || resp.ID == "" {
		return "", errors.Wrap(err, "Upload reading response")
	}

	return SnapshotID(resp.ID), nil
}

// Define the structure of the JSON
type snapshotPayload struct {
	View webview.View
}

type uploadTask struct {
	token    token.Token
	teamID   string
	snapshot snapshotPayload
}

func (t uploadTask) Empty() bool {
	return t.token == ""
}

func (s *SnapshotUploader) needsTask(st store.RStore) uploadTask {
	state := st.RLockState()
	defer st.RUnlockState()

	// Only upload if the feature is enabled.
	if !state.Features[feature.SnapshotsAutoUpload] {
		return uploadTask{}
	}

	// Only upload if there are new BuildCompleted events
	if state.CompletedBuildCount == 0 {
		return uploadTask{}
	}

	if state.CompletedBuildCount <= s.lastCompletedBuildCount {
		return uploadTask{}
	}

	// If we don't have an authenticated token, there's no
	// reason to even try to update.
	token := state.Token
	if token == "" {
		return uploadTask{}
	}

	teamID := state.TeamName
	view := webview.StateToWebView(state)
	s.lastCompletedBuildCount = state.CompletedBuildCount
	return uploadTask{
		token:    token,
		teamID:   teamID,
		snapshot: snapshotPayload{View: view},
	}
}

func (s *SnapshotUploader) runTask(ctx context.Context, t uploadTask) {
	buf := bytes.NewBuffer(nil)
	encoder := json.NewEncoder(buf)
	err := encoder.Encode(t.snapshot)
	if err != nil {
		logger.Get(ctx).Infof("Error auto-uploading snapshot: %v", err)
		return
	}

	id, err := s.Upload(t.token, t.teamID, buf)
	if err != nil {
		logger.Get(ctx).Infof("Error auto-uploading snapshot: %v", err)
		return
	}

	logger.Get(ctx).Infof("Uploaded snapshot (%s)", s.IDToSnapshotURL(id))
}

func (s *SnapshotUploader) OnChange(ctx context.Context, st store.RStore) {
	task := s.needsTask(st)
	if !task.Empty() {
		s.runTask(ctx, task)
	}
}

var _ store.Subscriber = &SnapshotUploader{}
