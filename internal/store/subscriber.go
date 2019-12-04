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

// Some subscribers need to do SetUp or TearDown.
// Both hold the subscriber lock, so should return quickly.
type SetUpper interface {
	SetUp(ctx context.Context)
}
type TearDowner interface {
	TearDown(ctx context.Context)
}

// Convenience interface for subscriber fulfilling both SetUpper and TearDowner
type SubscriberLifecycle interface {
	SetUpper
	TearDowner
}

type subscriberList struct {
	subscribers []*subscriberEntry
	setup       bool
	mu          sync.Mutex
}

func (l *subscriberList) Add(ctx context.Context, s Subscriber) {
	l.mu.Lock()
	defer l.mu.Unlock()

	e := &subscriberEntry{
		subscriber: s,
		dirtyBit:   NewDirtyBit(),
	}
	l.subscribers = append(l.subscribers, e)
	if l.setup {
		// the rest of the subscriberList has already been set up, so set up this subscriber directly
		e.maybeSetUp(ctx)
	}
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

func (l *subscriberList) SetUp(ctx context.Context) {
	l.mu.Lock()
	subscribers := append([]*subscriberEntry{}, l.subscribers...)
	l.setup = true
	l.mu.Unlock()

	for _, s := range subscribers {
		s.maybeSetUp(ctx)
	}
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
		s := s
		s.dirtyBit.MarkDirty()

		SafeGo(store, func() {
			s.notify(ctx, store)
		})
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

func (e *subscriberEntry) maybeSetUp(ctx context.Context) {
	s, ok := e.subscriber.(SetUpper)
	if ok {
		e.mu.Lock()
		defer e.mu.Unlock()
		s.SetUp(ctx)
	}
}

func (e *subscriberEntry) maybeTeardown(ctx context.Context) {
	s, ok := e.subscriber.(TearDowner)
	if ok {
		e.mu.Lock()
		defer e.mu.Unlock()
		s.TearDown(ctx)
	}
}
