package cloud

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/token"
)

type SnapshotID string

type SnapshotUploader struct {
	client HttpClient
	addr   Address
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

	if response.StatusCode != http.StatusOK {
		b, err := ioutil.ReadAll(response.Body)
		if err != nil {
			return "", fmt.Errorf("posting snapshot failed, and then reading snapshot response failed. status: %s, error: %v", response.Status, b)
		}
		return "", fmt.Errorf("posting snapshot failed. status: %s, response: %s", response.Status, b)
	}

	// unpack response with snapshot ID
	var resp snapshotIDResponse
	decoder := json.NewDecoder(response.Body)
	err = decoder.Decode(&resp)
	if err != nil || resp.ID == "" {
		return "", errors.Wrap(err, "Upload reading response")
	}

	return SnapshotID(resp.ID), nil
}
