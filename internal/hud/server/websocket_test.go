package server

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/testutils"

	"github.com/windmilleng/tilt/internal/store"
)

func TestWebsocketCloseOnReadErr(t *testing.T) {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	st, _ := store.NewStoreForTesting()
	st.SetUpSubscribersForTesting(ctx)

	conn := newFakeConn()
	ws := NewWebsocketSubscriber(conn)
	st.AddSubscriber(ctx, ws)

	done := make(chan bool)
	go func() {
		ws.Stream(ctx, st)
		close(done)
	}()

	st.NotifySubscribers(ctx)
	conn.AssertNextWriteMsg(t).Ack()

	st.NotifySubscribers(ctx)
	conn.AssertNextWriteMsg(t).Ack()

	conn.readCh <- fmt.Errorf("read error")

	conn.AssertClose(t, done)
}

func TestWebsocketReadErrDuringMsg(t *testing.T) {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	st, _ := store.NewStoreForTesting()
	st.SetUpSubscribersForTesting(ctx)

	conn := newFakeConn()
	ws := NewWebsocketSubscriber(conn)
	st.AddSubscriber(ctx, ws)

	done := make(chan bool)
	go func() {
		ws.Stream(ctx, st)
		close(done)
	}()

	st.NotifySubscribers(ctx)

	m := conn.AssertNextWriteMsg(t)

	// Send a read error, and make sure the connection
	// doesn't close immediately.
	conn.readCh <- fmt.Errorf("read error")
	time.Sleep(10 * time.Millisecond)
	assert.False(t, conn.closed)

	// Finish the write
	m.Ack()

	conn.AssertClose(t, done)
}

type fakeConn struct {
	// Write an error to this channel to stop the Read consumer
	readCh chan error

	// Consume messages written to this channel. The caller should Ack() to acknowledge receipt.
	writeCh chan msg

	closed bool
}

func newFakeConn() *fakeConn {
	return &fakeConn{
		readCh:  make(chan error),
		writeCh: make(chan msg),
	}
}

func (c *fakeConn) NextReader() (int, io.Reader, error) {
	return 1, nil, <-c.readCh
}

func (c *fakeConn) Close() error {
	c.closed = true
	return nil
}

func (c *fakeConn) WriteJSON(v interface{}) error {
	msg := msg{callback: make(chan error)}
	c.writeCh <- msg
	return <-msg.callback
}

func (c *fakeConn) AssertNextWriteMsg(t *testing.T) msg {
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for WriteJSON")
	case msg := <-c.writeCh:
		return msg
	}
	return msg{}
}

func (c *fakeConn) AssertClose(t *testing.T, done chan bool) {
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for close")
	case <-done:
		assert.True(t, c.closed)
	}
}

type msg struct {
	callback chan error
}

func (m msg) Ack() {
	m.callback <- nil
	close(m.callback)
}
