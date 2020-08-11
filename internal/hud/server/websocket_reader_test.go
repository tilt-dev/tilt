package server

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

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

	f.tearDown()
}

func TestIncrementalLogAck(t *testing.T) {
	f := newWebsocketReaderFixture(t)
	f.start()

	v := &proto_webview.View{Log: "hello world", LogList: &proto_webview.LogList{ToCheckpoint: 123}}
	f.sendView(v)

	f.assertHandlerCallCount(1)
	assert.Equal(t, "hello world", f.handler.lastViewLog)

	// Expect client to send an Ack, so make sure that that message was written to the conn
	f.assertMessageWritten()

	f.tearDown()
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

	f.tearDown()
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
	out     *bytes.Buffer
	conn    *fakeConn
	handler *fakeViewHandler
	wsr     *WebsocketReader
	done    chan error
}

func newWebsocketReaderFixture(t *testing.T) *websocketReaderFixture {
	out := new(bytes.Buffer)
	baseCtx, _, _ := testutils.ForkedCtxAndAnalyticsForTest(out)
	ctx, cancel := context.WithCancel(baseCtx)
	conn := newFakeConn()
	handler := &fakeViewHandler{}

	return &websocketReaderFixture{
		t:       t,
		ctx:     ctx,
		cancel:  cancel,
		out:     out,
		conn:    conn,
		handler: handler,
		wsr:     newWebsocketReader(conn, handler),
		done:    make(chan error),
	}
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
	err := f.wsr.marshaller.Marshal(buf, v)
	assert.NoError(f.t, err)

	f.conn.newMessageToRead(buf)
}

func (f *websocketReaderFixture) assertMessageWritten() {
	f.conn.AssertNextWriteMsg(f.t).Ack()
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
	ctx, cancel := context.WithTimeout(f.ctx, time.Millisecond*10)
	defer cancel()
	isCanceled := false

	for {
		logs := f.out.String()
		if strings.Contains(logs, msg) {
			return
		}
		if isCanceled {
			f.t.Fatalf("Timed out waiting for logs to contain message: %q\nLOGS:\n%s",
				msg, logs)
		}

		select {
		case <-ctx.Done():
			// Let the loop run the check one more time
			isCanceled = true
		case <-time.After(time.Millisecond):
		}
	}
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

func (fvh *fakeViewHandler) Handle(v proto_webview.View) error {
	fvh.callCount += 1
	if fvh.nextErr != nil {
		err := fvh.nextErr
		fvh.nextErr = nil
		return err
	}
	fvh.lastViewLog = v.Log
	return nil
}
