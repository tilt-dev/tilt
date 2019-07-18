package client

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/testutils"

	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/hud/webview"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

const (
	testRoomID = model.RoomID("some-room")
	testSecret = "shh-very-secret"
)

func TestConnectAndBroadcast(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	// Trying to broadcast before connecting does nothing
	f.client.OnChange(f.ctx, f.store)
	assert.Nil(t, f.client.conn)

	// Initial connect
	err := f.client.Connect(f.ctx, f.store)
	if err != nil {
		t.Fatal(err)
	}
	f.assertNewRoomCalls(1)
	assert.NotNil(t, f.conn()) // connection is established (nothing sent down it yet)

	f.client.OnChange(f.ctx, f.store)
	assert.Equal(t, 1, len(f.conn().json.(webview.View).Resources))
	assert.Equal(t, view.TiltfileResourceName, f.conn().json.(webview.View).Resources[0].Name.String())

	// Change state and broadcast again, see that number of resources updates to reflect new state
	state := f.store.LockMutableStateForTesting()
	state.UpsertManifestTarget(store.NewManifestTarget(model.Manifest{Name: "fe"}))
	f.store.UnlockMutableState()

	f.client.OnChange(f.ctx, f.store)
	assert.Equal(t, 2, len(f.conn().json.(webview.View).Resources))
	f.assertNewRoomCalls(1) // room already connected, shouldn't have any more NewRoom calls
}

func TestRoomConnectedWithVersion(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	state := f.store.LockMutableStateForTesting()
	state.TiltBuildInfo = model.TiltBuild{Version: "1.2.3"}
	f.store.UnlockMutableState()

	err := f.client.Connect(f.ctx, f.store)
	if err != nil {
		t.Fatal(err)
	}

	f.assertNewRoomLastVersion(model.WebVersion("v1.2.3"))
}

func TestSailRoomConnectedAction(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	go f.store.Loop(f.ctx)

	err := f.client.Connect(f.ctx, f.store)
	if err != nil {
		t.Fatal(err)
	}

	a := store.WaitForAction(t, reflect.TypeOf(SailRoomConnectedAction{}), f.getActions)
	if roomConn, ok := a.(SailRoomConnectedAction); ok {
		assert.NoError(t, roomConn.Err)
		assert.Equal(t, "http://localhost:12345/view/some-room", roomConn.ViewURL)
	}
}

type fixture struct {
	t          *testing.T
	ctx        context.Context
	cancel     func()
	client     *sailClient
	store      *store.Store
	getActions func() []store.Action
}

func newFixture(t *testing.T) *fixture {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)
	u, err := url.Parse("ws://localhost:12345")
	if err != nil {
		t.Fatal(err)
	}

	st, getActions := store.NewStoreForTesting()

	client := ProvideSailClient(model.SailURL(*u), &fakeSailRoomer{}, fakeSailDialer{})
	client.persistentCtx = ctx
	return &fixture{
		t:          t,
		ctx:        ctx,
		cancel:     cancel,
		client:     client,
		store:      st,
		getActions: getActions,
	}
}

func (f *fixture) conn() *fakeSailConn {
	if f.client.conn == nil {
		f.t.Fatal("client.conn is unexpectedly nil")
	}
	return f.client.conn.(*fakeSailConn)
}

func (f *fixture) assertNewRoomCalls(n int) {
	fakeRoomer, ok := f.client.roomer.(*fakeSailRoomer)
	if !ok {
		f.t.Fatal("client.roomer is not of type fakeSailRoomer??")
	}
	assert.Equal(f.t, n, fakeRoomer.newRoomCalls, "expected %d calls to NewRoom, got %d", n, fakeRoomer.newRoomCalls)
}

func (f *fixture) assertNewRoomLastVersion(v model.WebVersion) {
	fakeRoomer, ok := f.client.roomer.(*fakeSailRoomer)
	if !ok {
		f.t.Fatal("client.roomer is not of type fakeSailRoomer??")
	}
	assert.Equal(f.t, v, fakeRoomer.newRoomLastVersion,
		"expected most recent call NewRoom to be made with version %s, but got %v", v, fakeRoomer.newRoomLastVersion)
}

func (f *fixture) TearDown() {
	f.cancel()
}

type fakeSailRoomer struct {
	newRoomCalls       int
	newRoomLastVersion model.WebVersion
}

func (r *fakeSailRoomer) NewRoom(ctx context.Context, version model.WebVersion) (roomID model.RoomID, secret string, err error) {
	r.newRoomCalls += 1
	r.newRoomLastVersion = version
	return testRoomID, testSecret, nil
}

type fakeSailDialer struct{}

func (d fakeSailDialer) DialContext(ctx context.Context, addr string, headers http.Header) (SailConn, error) {
	return &fakeSailConn{ctx: ctx}, nil
}

type fakeSailConn struct {
	ctx    context.Context
	json   interface{}
	closed bool
}

func (c *fakeSailConn) WriteJSON(v interface{}) error {
	c.json = v
	return nil
}

func (c *fakeSailConn) NextReader() (int, io.Reader, error) {
	<-c.ctx.Done()
	return 0, nil, c.ctx.Err()
}

func (c *fakeSailConn) Close() error {
	c.closed = true
	return nil
}
