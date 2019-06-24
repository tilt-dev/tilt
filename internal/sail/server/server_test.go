package server

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/model"
)

const (
	testRoomID = model.RoomID("some-room")
	testSecret = "shh-very-secret"
)

func TestGetRoomWithAuth(t *testing.T) {
	serv := sailServerForTest()
	serv.rooms[testRoomID] = &Room{
		id:     testRoomID,
		secret: testSecret,
	}

	r, err := serv.getRoomWithAuth(testRoomID, testSecret)
	if assert.NoError(t, err) {
		assert.Equal(t, testRoomID, r.id)
	}

	_, err = serv.getRoomWithAuth(testRoomID, "wrong-secret")
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "incorrect secret")
	}

	_, err = serv.getRoomWithAuth("roomDNE", testSecret)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "no room found")
	}
}

func sailServerForTest() SailServer {
	return ProvideSailServer(fakeAssetServer{})
}

type fakeAssetServer struct{}

func (as fakeAssetServer) ServeHTTP(http.ResponseWriter, *http.Request) {}
func (as fakeAssetServer) Serve(ctx context.Context) error              { return nil }
func (as fakeAssetServer) TearDown(ctx context.Context)                 {}
