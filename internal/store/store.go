package store

import "sync"

// A central state store, modeled after the Reactive programming UX pattern.
// Terminology is borrowed liberally from Redux. These docs in particular are helpful:
// https://redux.js.org/introduction/threeprinciples
// https://redux.js.org/basics
type Store struct {
	state       *EngineState
	actionQueue *actionQueue
	actionCh    chan Action
	mu          sync.Mutex
	stateMu     sync.RWMutex

	// TODO(nick): Define Subscribers and Reducers.
	// The actionChan is an intermediate representation to make the transition easiser.
}

func NewStore() *Store {
	return &Store{
		state:       NewState(),
		actionQueue: &actionQueue{},
		actionCh:    make(chan Action),
	}
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

// TODO(nick): Phase this out. Anyone that uses this should be implemented as a reducer.
func (s *Store) LockMutableState() *EngineState {
	s.stateMu.Lock()
	return s.state
}

func (s *Store) UnlockMutableState() {
	s.stateMu.Unlock()
}

func (s *Store) Actions() <-chan Action {
	return s.actionCh
}

func (s *Store) Dispatch(action Action) {
	s.actionQueue.add(action)
	go s.drainActions()
}

func (s *Store) Close() {
	close(s.actionCh)
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
