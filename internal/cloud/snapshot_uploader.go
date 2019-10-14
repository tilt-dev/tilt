package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/token"
)

type SnapshotID string

// Auto-uploads a snapshot every time a build completes.
type SnapshotUploader struct {
	client                  HttpClient
	addr                    Address
	lastCompletedBuildCount int
}

func NewSnapshotUploader(client HttpClient, addr Address) SnapshotUploader {
	return SnapshotUploader{
		client: client,
		addr:   addr,
	}
}

func (s SnapshotUploader) newSnapshotURL() string {
	u := URL(string(s.addr))
	u.Path = "/api/snapshot/new"
	return u.String()
}

func (s SnapshotUploader) IDToSnapshotURL(id SnapshotID) string {
	u := URL(string(s.addr))
	u.Path = fmt.Sprintf("snapshot/%s", id)
	return u.String()
}

type snapshotIDResponse struct {
	ID string
}

func (s SnapshotUploader) Upload(token token.Token, teamID string, reader io.Reader) (SnapshotID, error) {
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

func (s SnapshotUploader) OnChange(ctx context.Context, st store.RStore) {
	// TODO(nick): Implement me.
}
