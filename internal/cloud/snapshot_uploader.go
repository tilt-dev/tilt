package cloud

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/grpc-ecosystem/grpc-gateway/runtime"
	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/cloud/cloudurl"
	"github.com/windmilleng/tilt/internal/hud/webview"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/token"
	proto_webview "github.com/windmilleng/tilt/pkg/webview"
)

type SnapshotID string

func ToSnapshot(state store.EngineState) (*proto_webview.Snapshot, error) {
	view, err := webview.StateToProtoView(state, 0)
	if err != nil {
		return nil, err
	}
	return &proto_webview.Snapshot{View: view}, nil
}

type SnapshotUploader interface {
	TakeAndUpload(state store.EngineState) (SnapshotID, error)
	Upload(token token.Token, teamID string, snapshot *proto_webview.Snapshot) (SnapshotID, error)
	IDToSnapshotURL(id SnapshotID) string
}

type snapshotUploader struct {
	client HttpClient
	addr   cloudurl.Address
}

func NewSnapshotUploader(client HttpClient, addr cloudurl.Address) SnapshotUploader {
	return snapshotUploader{
		client: client,
		addr:   addr,
	}
}

func (s snapshotUploader) newSnapshotURL() string {
	u := cloudurl.URL(string(s.addr))
	u.Path = "/api/snapshot/new"
	return u.String()
}

func (s snapshotUploader) IDToSnapshotURL(id SnapshotID) string {
	u := cloudurl.URL(string(s.addr))
	u.Path = fmt.Sprintf("snapshot/%s", id)
	return u.String()
}

type snapshotIDResponse struct {
	ID string
}

func (s snapshotUploader) TakeAndUpload(state store.EngineState) (SnapshotID, error) {
	snapshot, err := ToSnapshot(state)
	if err != nil {
		return "", err
	}
	return s.Upload(state.Token, state.TeamName, snapshot)
}

func (s snapshotUploader) Upload(token token.Token, teamID string, snapshot *proto_webview.Snapshot) (SnapshotID, error) {
	b := &bytes.Buffer{}
	jsEncoder := &runtime.JSONPb{OrigName: false, EmitDefaults: true}
	err := jsEncoder.NewEncoder(b).Encode(snapshot)
	if err != nil {
		return "", errors.Wrap(err, "encoding snapshot")
	}
	request, err := http.NewRequest(http.MethodPost, s.newSnapshotURL(), b)
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
