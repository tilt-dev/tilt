package store

import (
	"context"
	"fmt"
	"sync"
)

// A subscriber is notified whenever the state changes.
//
// Subscribers do not need to be thread-safe. The Store will only
// call OnChange for a given subscriber when the last call completes.
//
// Subscribers are only allowed to read state. If they want to
// modify state, they should call store.Dispatch()
type Subscriber interface {
	OnChange(ctx context.Context, st RStore)
}

// Some subscribers need to do teardown.
// Teardown holds the subscriber lock, so we expect it
// to return quickly.
// TODO(nick): A Setup method would also be useful for subscribers
// that do one-time setup.
type SubscriberLifecycle interface {
	Teardown(ctx context.Context)
}

type subscriberList struct {
	subscribers []*subscriberEntry
	setup       bool
	mu          sync.Mutex
}

func (l *subscriberList) Add(s Subscriber) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.subscribers = append(l.subscribers, &subscriberEntry{
		subscriber: s,
		dirtyBit:   NewDirtyBit(),
	})
}

func (l *subscriberList) Remove(ctx context.Context, s Subscriber) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	for i, current := range l.subscribers {
		if s == current.subscriber {
			l.subscribers = append(l.subscribers[:i], l.subscribers[i+1:]...)
			if l.setup {
				current.maybeTeardown(ctx)
			}
			return nil
		}
	}

	return fmt.Errorf("Subscriber not found: %T: %+v", s, s)
}

func (l *subscriberList) Setup(ctx context.Context) {
	l.mu.Lock()
	l.setup = true
	l.mu.Unlock()
}

func (l *subscriberList) TeardownAll(ctx context.Context) {
	l.mu.Lock()
	subscribers := append([]*subscriberEntry{}, l.subscribers...)
	l.setup = false
	l.mu.Unlock()

	for _, s := range subscribers {
		s.maybeTeardown(ctx)
	}
}

func (l *subscriberList) NotifyAll(ctx context.Context, store *Store) {
	l.mu.Lock()
	subscribers := append([]*subscriberEntry{}, l.subscribers...)
	l.mu.Unlock()

	for _, s := range subscribers {
		s.dirtyBit.MarkDirty()

		go s.notify(ctx, store)
	}
}

type subscriberEntry struct {
	subscriber Subscriber
	mu         sync.Mutex
	dirtyBit   *DirtyBit
}

func (e *subscriberEntry) notify(ctx context.Context, store *Store) {
	e.mu.Lock()
	defer e.mu.Unlock()

	startToken, isDirty := e.dirtyBit.StartBuildIfDirty()
	if !isDirty {
		return
	}

	e.subscriber.OnChange(ctx, store)
	e.dirtyBit.FinishBuild(startToken)
}

func (e *subscriberEntry) maybeTeardown(ctx context.Context) {
	sl, ok := e.subscriber.(SubscriberLifecycle)
	if ok {
		e.mu.Lock()
		defer e.mu.Unlock()
		sl.Teardown(ctx)
	}
}
