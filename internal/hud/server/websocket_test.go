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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
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
	ctrlClient := fake.NewFakeTiltClient()
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
	ctrlClient := fake.NewFakeTiltClient()
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
	ctrlClient := fake.NewFakeTiltClient()
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
	f := newWSFixture(t)
	ctx := f.ctx
	ws := f.ws
	st := f.st
	conn := f.conn

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

func TestMergeUpdates(t *testing.T) {
	f := newWSFixture(t)

	f.ws.SendUISessionUpdate(f.ctx,
		&v1alpha1.UISession{ObjectMeta: metav1.ObjectMeta{Name: "sa"}})
	f.ws.SendUISessionUpdate(f.ctx,
		&v1alpha1.UISession{ObjectMeta: metav1.ObjectMeta{Name: "sb"}})
	f.ws.SendUIResourceUpdate(f.ctx, types.NamespacedName{Name: "ra"},
		&v1alpha1.UIResource{ObjectMeta: metav1.ObjectMeta{Name: "ra"}})
	f.ws.SendUIResourceUpdate(f.ctx, types.NamespacedName{Name: "ra"},
		&v1alpha1.UIResource{ObjectMeta: metav1.ObjectMeta{Name: "ra"}})
	f.ws.SendUIResourceUpdate(f.ctx, types.NamespacedName{Name: "rb"}, nil)
	f.ws.SendUIButtonUpdate(f.ctx, types.NamespacedName{Name: "ba"},
		&v1alpha1.UIButton{ObjectMeta: metav1.ObjectMeta{Name: "ba"}})
	f.ws.SendUIButtonUpdate(f.ctx, types.NamespacedName{Name: "ba"},
		&v1alpha1.UIButton{ObjectMeta: metav1.ObjectMeta{Name: "ba"}})
	f.ws.SendUIButtonUpdate(f.ctx, types.NamespacedName{Name: "bb"}, nil)
	f.ws.SendClusterUpdate(f.ctx, types.NamespacedName{Name: "ca"},
		&v1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "ca"}})
	f.ws.SendClusterUpdate(f.ctx, types.NamespacedName{Name: "ca"},
		&v1alpha1.Cluster{ObjectMeta: metav1.ObjectMeta{Name: "ca"}})
	f.ws.SendClusterUpdate(f.ctx, types.NamespacedName{Name: "cb"}, nil)

	view := f.ws.toViewUpdate()
	assert.Equal(t, "sb", view.UiSession.ObjectMeta.Name)
	assert.Equal(t, 2, len(view.UiResources))
	assert.Equal(t, 2, len(view.UiButtons))
	assert.Len(t, view.Clusters, 2, "Cluster updates")

	view2 := f.ws.toViewUpdate()
	assert.Nil(t, view2)

	f.ws.SendUIButtonUpdate(f.ctx, types.NamespacedName{Name: "bb"}, nil)
	view3 := f.ws.toViewUpdate()
	assert.Nil(t, view3.UiSession)
	assert.Equal(t, 0, len(view3.UiResources))
	assert.Equal(t, 1, len(view3.UiButtons))

	f.ws.SendClusterUpdate(f.ctx, types.NamespacedName{Name: "cb"}, nil)
	view4 := f.ws.toViewUpdate()
	assert.Len(t, view4.Clusters, 1, "Cluster updates")
}

type wsFixture struct {
	ws   *WebsocketSubscriber
	ctx  context.Context
	st   *store.Store
	conn *fakeConn
}

func newWSFixture(t *testing.T) *wsFixture {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	st, _ := store.NewStoreWithFakeReducer()
	_ = st.SetUpSubscribersForTesting(ctx)

	conn := newFakeConn()
	ctrlClient := fake.NewFakeTiltClient()
	ws := NewWebsocketSubscriber(ctx, ctrlClient, st, conn)
	require.NoError(t, st.AddSubscriber(ctx, ws))
	return &wsFixture{
		ctx:  ctx,
		st:   st,
		ws:   ws,
		conn: conn,
	}
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
