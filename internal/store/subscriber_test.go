package store

import (
	"context"
	"testing"
	"time"
)

func TestSubscriber(t *testing.T) {
	st := NewStore(EmptyReducer)
	ctx := context.Background()
	s := newFakeSubscriber()
	st.AddSubscriber(s)

	st.NotifySubscribers(ctx)
	call := <-s.onChange
	close(call.done)
}

func TestSubscriberInterleavedCalls(t *testing.T) {
	st := NewStore(EmptyReducer)
	ctx := context.Background()
	s := newFakeSubscriber()
	st.AddSubscriber(s)

	st.NotifySubscribers(ctx)
	call := <-s.onChange
	st.NotifySubscribers(ctx)
	st.NotifySubscribers(ctx)
	close(call.done)

	call = <-s.onChange
	close(call.done)
	call = <-s.onChange
	close(call.done)

	select {
	case <-s.onChange:
		t.Fatal("Expected no more onChange calls")
	case <-time.After(10 * time.Millisecond):
	}
}

type fakeSubscriber struct {
	onChange chan onChangeCall
}

func newFakeSubscriber() fakeSubscriber {
	return fakeSubscriber{
		onChange: make(chan onChangeCall),
	}
}

type onChangeCall struct {
	done chan bool
}

func (f fakeSubscriber) OnChange(ctx context.Context, store *Store) {
	call := onChangeCall{done: make(chan bool)}
	f.onChange <- call
	<-call.done
}
