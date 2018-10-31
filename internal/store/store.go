package store

import (
	"context"
	"sync"
)

// A central state store, modeled after the Reactive programming UX pattern.
// Terminology is borrowed liberally from Redux. These docs in particular are helpful:
// https://redux.js.org/introduction/threeprinciples
// https://redux.js.org/basics
type Store struct {
	state       *EngineState
	subscribers *subscriberList
	actionQueue *actionQueue
	actionCh    chan Action
	mu          sync.Mutex
	stateMu     sync.RWMutex
	reduce      Reducer

	// TODO(nick): Define Subscribers and Reducers.
	// The actionChan is an intermediate representation to make the transition easiser.
}

func NewStore(reducer Reducer) *Store {
	return &Store{
		state:       NewState(),
		reduce:      reducer,
		actionQueue: &actionQueue{},
		actionCh:    make(chan Action),
		subscribers: &subscriberList{},
	}
}

func NewStoreForTesting() *Store {
	return NewStore(EmptyReducer)
}

func (s *Store) AddSubscriber(sub Subscriber) {
	s.subscribers.Add(sub)
}

// Sends messages to all the subscribers asynchronously.
func (s *Store) NotifySubscribers(ctx context.Context) {
	s.subscribers.NotifyAll(ctx, s)
}

// TODO(nick): Clone the state to ensure it's not mutated.
// For now, we use RW locks to simulate the same behavior, but the
// onus is on the caller to RUnlockState.
func (s *Store) RLockState() EngineState {
	s.stateMu.RLock()
	return *(s.state)
}

func (s *Store) RUnlockState() {
	s.stateMu.RUnlock()
}

func (s *Store) LockMutableStateForTesting() *EngineState {
	s.stateMu.Lock()
	return s.state
}

func (s *Store) UnlockMutableState() {
	s.stateMu.Unlock()
}

func (s *Store) Dispatch(action Action) {
	s.actionQueue.add(action)
	go s.drainActions()
}

func (s *Store) Close() {
	close(s.actionCh)
}

func (s *Store) Loop(ctx context.Context) error {

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case action := <-s.actionCh:
			s.stateMu.Lock()
			s.reduce(ctx, s.state, action)
			s.stateMu.Unlock()
		}

		// Subscribers
		done, err := s.maybeFinished()
		if done {
			return err
		}
		s.NotifySubscribers(ctx)
	}
}

func (s *Store) maybeFinished() (bool, error) {
	state := s.RLockState()
	defer s.RUnlockState()

	if len(state.ManifestStates) == 0 {
		return false, nil
	}

	if state.PermanentError != nil {
		return true, state.PermanentError
	}

	finished := !state.WatchMounts && len(state.ManifestsToBuild) == 0 && state.CurrentlyBuilding == ""
	return finished, nil
}

func (s *Store) drainActions() {
	// The mutex here ensures that the actions appear on the channel in-order.
	// It will also be necessary once we have reducers.
	s.mu.Lock()
	defer s.mu.Unlock()

	actions := s.actionQueue.drain()
	for _, action := range actions {
		s.actionCh <- action
	}
}

type Action interface {
	Action()
}

type actionQueue struct {
	actions []Action
	mu      sync.Mutex
}

func (q *actionQueue) add(action Action) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.actions = append(q.actions, action)
}

func (q *actionQueue) drain() []Action {
	q.mu.Lock()
	defer q.mu.Unlock()
	result := append([]Action{}, q.actions...)
	q.actions = nil
	return result
}
