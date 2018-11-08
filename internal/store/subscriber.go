package store

import (
	"context"
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
	OnChange(ctx context.Context, dsr DispatchingStateReader)
}

type subscriberEntry struct {
	subscriber Subscriber
	mu         sync.Mutex
}

func (l *subscriberList) Add(s Subscriber) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.subscribers = append(l.subscribers, &subscriberEntry{
		subscriber: s,
	})
}

func (l *subscriberList) NotifyAll(ctx context.Context, store *Store) {
	l.mu.Lock()
	subscribers := append([]*subscriberEntry{}, l.subscribers...)
	l.mu.Unlock()

	for _, s := range subscribers {
		go s.notify(ctx, store)
	}
}

type subscriberList struct {
	subscribers []*subscriberEntry
	mu          sync.Mutex
}

func (e *subscriberEntry) notify(ctx context.Context, store *Store) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.subscriber.OnChange(ctx, store)
}
