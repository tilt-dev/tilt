package client

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/model"
	sailCommon "github.com/windmilleng/tilt/internal/sail/common"
)

type SailRoomer interface {
	NewRoom(ctx context.Context) (roomID, secret string, err error)
}

type httpRoomer struct {
	addr model.SailURL
}

func (r httpRoomer) NewRoom(ctx context.Context) (roomID, secret string, err error) {
	u := r.addr.Http()
	u.Path = "/new_room"
	resp, err := http.Get(u.String())
	if err != nil {
		return "", "", errors.Wrapf(err, "GET %s", u.String())
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", "", errors.Wrap(err, "reading /new_room response")
	}

	var roomInfo sailCommon.RoomInfo
	err = json.Unmarshal(body, &roomInfo)
	if err != nil {
		return "", "", errors.Wrapf(err, "unmarshaling json: %s", string(body))
	}

	return roomInfo.RoomID, roomInfo.Secret, nil
}

func ProvideSailRoomer(addr model.SailURL) SailRoomer {
	return httpRoomer{addr}
}
