package server

import (
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/model"
)

func TestNoFans(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.source.dataCh <- "hello"
	f.source.dataCh <- "goodbye"
}

func TestOneFan(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	fan := f.addFan()

	f.source.dataCh <- "hello"
	f.source.dataCh <- "goodbye"

	assert.Equal(t, "hello", fan.nextMessage(t))
	assert.Equal(t, "goodbye", fan.nextMessage(t))
}

func TestDisconnectOneFan(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	fan := f.addFan()

	f.source.dataCh <- "hello"
	assert.Equal(t, "hello", fan.nextMessage(t))

	fan.readErrCh <- fmt.Errorf("disconnect")
	time.Sleep(10 * time.Millisecond)

	f.source.dataCh <- "goodbye"

	fan.assertNoNextMessage(t)
}

func TestTwoFans(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	fanA := f.addFan()
	fanB := f.addFan()

	f.source.dataCh <- "hello"
	f.source.dataCh <- "goodbye"

	assert.Equal(t, "hello", fanA.nextMessage(t))
	assert.Equal(t, "hello", fanB.nextMessage(t))
	assert.Equal(t, "goodbye", fanA.nextMessage(t))
	assert.Equal(t, "goodbye", fanB.nextMessage(t))
}

func TestNewFanGetsMostRecentMessage(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.source.dataCh <- "hello"
	f.source.dataCh <- "goodbye"
	time.Sleep(10 * time.Millisecond)

	fan := f.addFan()

	assert.Equal(t, "goodbye", fan.nextMessage(t))
}

type fixture struct {
	t      *testing.T
	ctx    context.Context
	cancel func()
	source *fakeSource
	room   *Room
	errCh  chan error
}

func newFixture(t *testing.T) *fixture {
	ctx, cancel := context.WithCancel(context.Background())
	source := newFakeSource(ctx)
	room := NewRoom(model.WebVersion("v1.2.3"))
	room.source = source
	errCh := make(chan error)
	go func() {
		errCh <- room.ConsumeSource(ctx)
		close(errCh)
	}()

	return &fixture{
		t:      t,
		ctx:    ctx,
		cancel: cancel,
		source: source,
		room:   room,
		errCh:  errCh,
	}
}

func (f *fixture) addFan() *fakeFan {
	fan := newFakeFan(f.ctx)
	f.room.AddFan(f.ctx, fan)
	return fan
}

func (f *fixture) TearDown() {
	f.cancel()
	for err := range f.errCh {
		if err != nil && err != context.Canceled {
			f.t.Fatalf("ConsumeSource: %v", err)
		}
	}
}

type fakeSource struct {
	ctx    context.Context
	dataCh chan string
}

func newFakeSource(ctx context.Context) *fakeSource {
	return &fakeSource{
		ctx:    ctx,
		dataCh: make(chan string),
	}
}

func (s *fakeSource) ReadMessage() (int, []byte, error) {
	select {
	case <-s.ctx.Done():
		return 0, nil, s.ctx.Err()
	case data := <-s.dataCh:
		return websocket.TextMessage, []byte(data), nil
	}
}

func (f *fakeSource) Close() error {
	return nil
}

type fakeFan struct {
	ctx       context.Context
	dataCh    chan string
	readErrCh chan error
}

func newFakeFan(ctx context.Context) *fakeFan {
	return &fakeFan{
		ctx:       ctx,
		dataCh:    make(chan string, 0),
		readErrCh: make(chan error, 1),
	}
}

func (f *fakeFan) assertNoNextMessage(t *testing.T) {
	select {
	case <-time.After(10 * time.Millisecond):
	case data, ok := <-f.dataCh:
		if ok {
			t.Fatalf("Unexpected message: %v", data)
		}
	}
}

func (f *fakeFan) nextMessage(t *testing.T) string {
	select {
	case <-time.After(100 * time.Millisecond):
		t.Fatal("Message never arrived at fan")
	case data := <-f.dataCh:
		return data
	}
	return ""
}

func (f *fakeFan) WriteMessage(messageType int, data []byte) error {
	select {
	case f.dataCh <- string(data):
		return nil
	case <-f.ctx.Done():
		return f.ctx.Err()
	}
}

func (f *fakeFan) NextReader() (int, io.Reader, error) {
	select {
	case <-f.ctx.Done():
		return 0, nil, f.ctx.Err()
	case err := <-f.readErrCh:
		return 0, nil, err
	}
}

func (f *fakeFan) Close() error {
	close(f.dataCh)
	return nil
}
