package client

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/hud/webview"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils/output"
)

func TestBroadcast(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.client.OnChange(f.ctx, f.store)

	assert.Equal(t, 1, len(f.conn().json.(webview.View).Resources))
	assert.Equal(t, view.TiltfileResourceName, f.conn().json.(webview.View).Resources[0].Name.String())

	state := f.store.LockMutableStateForTesting()
	state.UpsertManifestTarget(store.NewManifestTarget(model.Manifest{Name: "fe"}))
	f.store.UnlockMutableState()

	f.client.OnChange(f.ctx, f.store)
	assert.Equal(t, 2, len(f.conn().json.(webview.View).Resources))
}

type fixture struct {
	t      *testing.T
	ctx    context.Context
	cancel func()
	client *SailClient
	store  *store.Store
}

func newFixture(t *testing.T) *fixture {
	ctx, cancel := context.WithCancel(output.CtxForTest())
	u, err := url.Parse("ws://localhost:12345")
	if err != nil {
		t.Fatal(err)
	}

	st, _ := store.NewStoreForTesting()

	client := ProvideSailClient(fakeSailDialer{}, model.SailURL(*u))
	return &fixture{
		t:      t,
		ctx:    ctx,
		cancel: cancel,
		client: client,
		store:  st,
	}
}

func (f *fixture) conn() *fakeSailConn {
	if f.client.conn == nil {
		f.t.Fatal("client.conn is unexpectedly nil")
	}
	return f.client.conn.(*fakeSailConn)
}

func (f *fixture) TearDown() {
	f.cancel()
}

type fakeSailDialer struct {
}

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
