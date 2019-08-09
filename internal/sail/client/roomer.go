package client

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/pkg/model"
)

// For injecting room creation logic (because the real way involves an HTTP request)
type SailRoomer interface {
	NewRoom(ctx context.Context, version model.WebVersion) (roomID model.RoomID, secret string, err error)
}

type httpRoomer struct {
	addr model.SailURL
}

func (r httpRoomer) NewRoom(ctx context.Context, version model.WebVersion) (roomID model.RoomID, secret string, err error) {
	u := r.addr.Http()
	u.Path = "/room"

	req := model.SailNewRoomRequest{WebVersion: version}
	reqJson, err := json.Marshal(req)
	if err != nil {
		return "", "", errors.Wrap(err, "json.Marshal new room request")
	}

	resp, err := http.Post(u.String(), "text/plain", bytes.NewReader(reqJson))
	if err != nil {
		return "", "", err
	}

	var roomInfo model.SailRoomInfo
	err = json.NewDecoder(resp.Body).Decode(&roomInfo)
	if err != nil {
		return "", "", errors.Wrap(err, "json-decoding POST /room response body")
	}

	return roomInfo.RoomID, roomInfo.Secret, nil
}

func ProvideSailRoomer(addr model.SailURL) SailRoomer {
	return httpRoomer{addr}
}
