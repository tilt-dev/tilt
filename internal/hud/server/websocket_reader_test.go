package server

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/golang/protobuf/jsonpb"
	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/testutils/bufsync"

	"github.com/tilt-dev/tilt/internal/testutils"
	proto_webview "github.com/tilt-dev/tilt/pkg/webview"
)

func TestViewsHandled(t *testing.T) {
	f := newWebsocketReaderFixture(t)
	f.start()

	v := &proto_webview.View{Log: "hello world"}
	f.sendView(v)
	f.assertHandlerCallCount(1)
	assert.Equal(t, "hello world", f.handler.lastViewLog)

	v = &proto_webview.View{Log: "goodbye world"}
	f.sendView(v)
	f.assertHandlerCallCount(2)
	assert.Equal(t, "goodbye world", f.handler.lastViewLog)
}

func TestHandlerErrorDoesntStopLoop(t *testing.T) {
	f := newWebsocketReaderFixture(t)
	f.start()
	f.handler.nextErr = fmt.Errorf("aw nerts")

	v := &proto_webview.View{Log: "hello world"}
	f.sendView(v)
	f.assertHandlerCallCount(1)
	f.assertLogs("aw nerts")

	// should still be running!
	v = &proto_webview.View{Log: "goodbye world"}
	f.sendView(v)
	f.assertHandlerCallCount(2)
	assert.Equal(t, "goodbye world", f.handler.lastViewLog)
}

func TestNonPersistentReaderExistsAfterHandling(t *testing.T) {
	f := newWebsocketReaderFixture(t).withPersistent(false)
	f.start()

	v := &proto_webview.View{Log: "hello world"}
	f.sendView(v)
	f.assertHandlerCallCount(1)
	assert.Equal(t, "hello world", f.handler.lastViewLog)
	f.assertDone()
}

func TestWebsocketCloseOnNextReaderError(t *testing.T) {
	f := newWebsocketReaderFixture(t)
	f.start()

	f.conn.readCh <- readerOrErr{err: fmt.Errorf("read error")}

	time.Sleep(10 * time.Millisecond)
	f.assertDone()
}

type websocketReaderFixture struct {
	t       *testing.T
	ctx     context.Context
	cancel  context.CancelFunc
	out     *bufsync.ThreadSafeBuffer
	conn    *fakeConn
	handler *fakeViewHandler
	wsr     *WebsocketReader
	done    chan error
}

func newWebsocketReaderFixture(t *testing.T) *websocketReaderFixture {
	out := bufsync.NewThreadSafeBuffer()
	baseCtx, _, _ := testutils.ForkedCtxAndAnalyticsForTest(out)
	ctx, cancel := context.WithCancel(baseCtx)
	conn := newFakeConn()
	handler := &fakeViewHandler{}

	ret := &websocketReaderFixture{
		t:       t,
		ctx:     ctx,
		cancel:  cancel,
		out:     out,
		conn:    conn,
		handler: handler,
		wsr:     newWebsocketReader(conn, true, handler),
		done:    make(chan error),
	}

	t.Cleanup(ret.tearDown)
	return ret
}

func (f *websocketReaderFixture) withPersistent(persistent bool) *websocketReaderFixture {
	f.wsr.persistent = persistent
	return f
}

func (f *websocketReaderFixture) start() {
	go func() {
		err := f.wsr.Listen(f.ctx)
		f.done <- err
		close(f.done)
	}()
}

func (f *websocketReaderFixture) sendView(v *proto_webview.View) {
	buf := &bytes.Buffer{}
	err := (&jsonpb.Marshaler{}).Marshal(buf, v)
	assert.NoError(f.t, err)

	f.conn.newMessageToRead(buf)
}

func (f *websocketReaderFixture) assertHandlerCallCount(n int) {
	ctx, cancel := context.WithTimeout(f.ctx, time.Millisecond*10)
	defer cancel()
	isCanceled := false

	for {
		if f.handler.callCount == n {
			return
		}
		if isCanceled {
			f.t.Fatalf("Timed out waiting for handler.callCount = %d (got: %d)",
				n, f.handler.callCount)
		}

		select {
		case <-ctx.Done():
			// Let the loop run the check one more time
			isCanceled = true
		case <-time.After(time.Millisecond):
		}
	}
}

func (f *websocketReaderFixture) assertLogs(msg string) {
	f.out.AssertEventuallyContains(f.t, msg, time.Second)
}

func (f *websocketReaderFixture) tearDown() {
	f.cancel()
	f.assertDone()
}

func (f *websocketReaderFixture) assertDone() {
	select {
	case <-time.After(100 * time.Millisecond):
		f.t.Fatal("timed out waiting for close")
	case err := <-f.done:
		assert.NoError(f.t, err)
	}
}

type fakeViewHandler struct {
	callCount   int
	lastViewLog string // use the Log field to differentiate the views we send, cuz why not
	nextErr     error
}

func (fvh *fakeViewHandler) Handle(v *proto_webview.View) error {
	fvh.callCount += 1
	if fvh.nextErr != nil {
		err := fvh.nextErr
		fvh.nextErr = nil
		return err
	}
	fvh.lastViewLog = v.Log
	return nil
}
