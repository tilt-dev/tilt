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

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/pkg/logger"
)

func TestWebsocketCloseOnReadErr(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TODO(nick): investigate")
	}
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	st, _ := store.NewStoreWithFakeReducer()
	_ = st.SetUpSubscribersForTesting(ctx)

	conn := newFakeConn()
	ctrlClient := fake.NewTiltClient()
	ws := NewWebsocketSubscriber(ctx, ctrlClient, st, conn)
	require.NoError(t, st.AddSubscriber(ctx, ws))

	done := make(chan bool)
	go func() {
		ws.Stream(ctx)
		_ = st.RemoveSubscriber(context.Background(), ws)
		close(done)
	}()

	conn.AssertNextWriteMsg(t).Ack()

	writeLogAndNotify(ctx, st)
	conn.AssertNextWriteMsg(t).Ack()

	writeLogAndNotify(ctx, st)
	conn.AssertNextWriteMsg(t).Ack()

	conn.readCh <- readerOrErr{err: fmt.Errorf("read error")}

	conn.AssertClose(t, done)
}

func TestWebsocketReadErrDuringMsg(t *testing.T) {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	st, _ := store.NewStoreWithFakeReducer()
	_ = st.SetUpSubscribersForTesting(ctx)

	conn := newFakeConn()
	ctrlClient := fake.NewTiltClient()
	ws := NewWebsocketSubscriber(ctx, ctrlClient, st, conn)
	require.NoError(t, st.AddSubscriber(ctx, ws))

	done := make(chan bool)
	go func() {
		ws.Stream(ctx)
		_ = st.RemoveSubscriber(context.Background(), ws)
		close(done)
	}()

	conn.AssertNextWriteMsg(t).Ack()

	writeLogAndNotify(ctx, st)

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
	ctrlClient := fake.NewTiltClient()
	ws := NewWebsocketSubscriber(ctx, ctrlClient, st, conn)
	require.NoError(t, st.AddSubscriber(ctx, ws))

	done := make(chan bool)
	go func() {
		ws.Stream(ctx)
		_ = st.RemoveSubscriber(context.Background(), ws)
		close(done)
	}()

	writeLogAndNotify(ctx, st)
	time.Sleep(10 * time.Millisecond)

	conn.readCh <- readerOrErr{err: fmt.Errorf("read error")}
	conn.AssertClose(t, done)
}

// It's possible to get a ChangeSummary where Log is true but all logs have already been processed,
// in which case ToLogList returns [-1,-1).
// Presumably this happens when:
// 1. store writes logevent A to logstore
// 2. store notifies subscribers with a changesummary indicating there are logs
// 3. store writes logevent B to logstore
// 4. subscriber gets the changesummary from (2) and reads logevents A and B
// 5. store notifies subscribers of logevent B
// 6. subscriber reads logevents, but its checkpoint is already all caught up
// https://github.com/tilt-dev/tilt/issues/4604
func TestWebsocketIgnoreEmptyLogList(t *testing.T) {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	st, _ := store.NewStoreWithFakeReducer()
	_ = st.SetUpSubscribersForTesting(ctx)

	conn := newFakeConn()
	ctrlClient := fake.NewTiltClient()
	ws := NewWebsocketSubscriber(ctx, ctrlClient, st, conn)
	require.NoError(t, st.AddSubscriber(ctx, ws))

	done := make(chan bool)
	go func() {
		ws.Stream(ctx)
		_ = st.RemoveSubscriber(context.Background(), ws)
		close(done)
	}()

	conn.AssertNextWriteMsg(t).Ack()

	_ = ws.OnChange(ctx, st, store.ChangeSummary{Log: true})
	require.NotEqual(t, -1, ws.clientCheckpoint)
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

func writeLogAndNotify(ctx context.Context, st *store.Store) {
	state := st.LockMutableStateForTesting()
	state.LogStore.Append(store.NewGlobalLogAction(logger.InfoLvl, []byte("test")), nil)
	st.UnlockMutableState()
	st.NotifySubscribers(ctx, store.ChangeSummary{Log: true})
}
