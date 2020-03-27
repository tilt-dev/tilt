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
		isPending := s.claimPending()
		if isPending {
			SafeGo(store, func() {
				s.notify(ctx, store)
			})
		}
	}
}

type subscriberEntry struct {
	subscriber Subscriber

	// At any given time, there are at most two goroutines
	// notifying the subscriber: a pending goroutine and an active goroutine.
	hasPending bool
	hasActive  bool

	// The active mutex is held by the goroutine currently notifying the
	// subscriber. It may be held for a long time if the subscriber
	// takes a long time.
	activeMu sync.Mutex

	// The state mutex is just for updating the hasPending/hasActive state.
	// It should never be held a long time.
	stateMu sync.Mutex
}

// Returns true if this is the pending goroutine.
// Returns false to do nothing.
func (e *subscriberEntry) claimPending() bool {
	e.stateMu.Lock()
	defer e.stateMu.Unlock()

	if e.hasPending {
		return false
	}
	e.hasPending = true
	return true
}

func (e *subscriberEntry) movePendingToActive() {
	e.stateMu.Lock()
	defer e.stateMu.Unlock()

	e.hasPending = false
	e.hasActive = true
}

func (e *subscriberEntry) clearActive() {
	e.stateMu.Lock()
	defer e.stateMu.Unlock()

	e.hasActive = false
}

func (e *subscriberEntry) notify(ctx context.Context, store *Store) {
	e.activeMu.Lock()
	defer e.activeMu.Unlock()

	e.movePendingToActive()
	e.subscriber.OnChange(ctx, store)
	e.clearActive()
}

func (e *subscriberEntry) maybeSetUp(ctx context.Context) {
	s, ok := e.subscriber.(SetUpper)
	if ok {
		e.activeMu.Lock()
		defer e.activeMu.Unlock()
		s.SetUp(ctx)
	}
}

func (e *subscriberEntry) maybeTeardown(ctx context.Context) {
	s, ok := e.subscriber.(TearDowner)
	if ok {
		e.activeMu.Lock()
		defer e.activeMu.Unlock()
		s.TearDown(ctx)
	}
}
