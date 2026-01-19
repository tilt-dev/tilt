package fakeconn

import (
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type ReaderOrErr struct {
	Reader io.Reader
	Err    error
}

type FakeConn struct {
	// Write an error to this channel to stop the Read consumer
	ReadCh chan ReaderOrErr

	// Consume messages written to this channel. The caller should Ack() to acknowledge receipt.
	WriteCh chan Msg

	Closed bool

	NextWriterError error
}

func NewFakeConn() *FakeConn {
	return &FakeConn{
		ReadCh:  make(chan ReaderOrErr),
		WriteCh: make(chan Msg),
	}
}

func (c *FakeConn) NextReader() (int, io.Reader, error) {
	next := <-c.ReadCh
	return 1, next.Reader, next.Err
}

func (c *FakeConn) Close() error {
	c.Closed = true
	return nil
}

func (c *FakeConn) NewMessageToRead(r io.Reader) {
	c.ReadCh <- ReaderOrErr{Reader: r}
}

func (c *FakeConn) AssertNextWriteMsg(t *testing.T) Msg {
	t.Helper()
	select {
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for Writer to Close")
	case msg := <-c.WriteCh:
		return msg
	}
	return Msg{}
}

func (c *FakeConn) AssertClose(t *testing.T, done chan bool) {
	t.Helper()
	select {
	case <-time.After(250 * time.Millisecond):
		t.Fatal("timed out waiting for close")
	case <-done:
		assert.True(t, c.Closed)
	}
}

func (c *FakeConn) NextWriter(messagetype int) (io.WriteCloser, error) {
	if c.NextWriterError != nil {
		return nil, c.NextWriterError
	}
	return c.writer(), nil
}

func (c *FakeConn) writer() io.WriteCloser {
	return &fakeConnWriter{c: c}
}

type fakeConnWriter struct {
	c *FakeConn
}

func (f *fakeConnWriter) Write(p []byte) (int, error) {
	return len(p), nil
}

func (f *fakeConnWriter) Close() error {
	cb := make(chan error)
	f.c.WriteCh <- Msg{Callback: cb}
	return <-cb
}

type Msg struct {
	Callback chan error
}

func (m Msg) Ack() {
	m.Callback <- nil
	close(m.Callback)
}
