package server

import (
	"context"
	"fmt"
	"io"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/testutils"

	"github.com/tilt-dev/tilt/internal/store"
)

func TestWebsocketCloseOnReadErr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TODO(nick): investigate")
	}
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	st, _ := store.NewStoreWithFakeReducer()
	_ = st.SetUpSubscribersForTesting(ctx)

	conn := newFakeConn()
	ws := NewWebsocketSubscriber(ctx, st, conn)
	require.NoError(t, st.AddSubscriber(ctx, ws))

	done := make(chan bool)
	go func() {
		ws.Stream(ctx, st)
		_ = st.RemoveSubscriber(context.Background(), ws)
		close(done)
	}()

	conn.AssertNextWriteMsg(t).Ack()

	st.NotifySubscribers(ctx, store.ChangeSummary{Log: true})
	conn.AssertNextWriteMsg(t).Ack()

	st.NotifySubscribers(ctx, store.ChangeSummary{Log: true})
	conn.AssertNextWriteMsg(t).Ack()

	conn.readCh <- readerOrErr{err: fmt.Errorf("read error")}

	conn.AssertClose(t, done)
}

func TestWebsocketReadErrDuringMsg(t *testing.T) {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	st, _ := store.NewStoreWithFakeReducer()
	_ = st.SetUpSubscribersForTesting(ctx)

	conn := newFakeConn()
	ws := NewWebsocketSubscriber(ctx, st, conn)
	require.NoError(t, st.AddSubscriber(ctx, ws))

	done := make(chan bool)
	go func() {
		ws.Stream(ctx, st)
		_ = st.RemoveSubscriber(context.Background(), ws)
		close(done)
	}()

	conn.AssertNextWriteMsg(t).Ack()

	st.NotifySubscribers(ctx, store.ChangeSummary{Log: true})

	m := conn.AssertNextWriteMsg(t)

	// Send a read error, and make sure the connection
	// doesn't close immediately.
	conn.readCh <- readerOrErr{err: fmt.Errorf("read error")}
	time.Sleep(10 * time.Millisecond)
	assert.False(t, conn.closed)

	// Finish the write
	m.Ack()

	conn.AssertClose(t, done)
}

func TestWebsocketNextWriterError(t *testing.T) {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	st, _ := store.NewStoreWithFakeReducer()
	_ = st.SetUpSubscribersForTesting(ctx)

	conn := newFakeConn()
	conn.nextWriterError = fmt.Errorf("fake NextWriter error")
	ws := NewWebsocketSubscriber(ctx, st, conn)
	require.NoError(t, st.AddSubscriber(ctx, ws))

	done := make(chan bool)
	go func() {
		ws.Stream(ctx, st)
		_ = st.RemoveSubscriber(context.Background(), ws)
		close(done)
	}()

	st.NotifySubscribers(ctx, store.ChangeSummary{Log: true})
	time.Sleep(10 * time.Millisecond)

	conn.readCh <- readerOrErr{err: fmt.Errorf("read error")}
	conn.AssertClose(t, done)
}

type readerOrErr struct {
	reader io.Reader
	err    error
}
type fakeConn struct {
	// Write an error to this channel to stop the Read consumer
	readCh chan readerOrErr

	// Consume messages written to this channel. The caller should Ack() to acknowledge receipt.
	writeCh chan msg

	closed bool

	nextWriterError error
}

func newFakeConn() *fakeConn {
	return &fakeConn{
		readCh:  make(chan readerOrErr),
		writeCh: make(chan msg),
	}
}

func (c *fakeConn) NextReader() (int, io.Reader, error) {
	next := <-c.readCh
	return 1, next.reader, next.err
}

func (c *fakeConn) Close() error {
	c.closed = true
	return nil
}

func (c *fakeConn) newMessageToRead(r io.Reader) {
	c.readCh <- readerOrErr{reader: r}
}

func (c *fakeConn) AssertNextWriteMsg(t *testing.T) msg {
	select {
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for Writer to Close")
	case msg := <-c.writeCh:
		return msg
	}
	return msg{}
}

func (c *fakeConn) AssertClose(t *testing.T, done chan bool) {
	t.Helper()
	select {
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for close")
	case <-done:
		assert.True(t, c.closed)
	}
}

func (c *fakeConn) NextWriter(messagetype int) (io.WriteCloser, error) {
	if c.nextWriterError != nil {
		return nil, c.nextWriterError
	}
	return c.writer(), nil
}

func (c *fakeConn) writer() io.WriteCloser {
	return &fakeConnWriter{c: c}
}

type fakeConnWriter struct {
	c *fakeConn
}

func (f *fakeConnWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (f *fakeConnWriter) Close() error {
	cb := make(chan error)
	f.c.writeCh <- msg{callback: cb}
	return <-cb
}

type msg struct {
	callback chan error
}

func (m msg) Ack() {
	m.callback <- nil
	close(m.callback)
}
