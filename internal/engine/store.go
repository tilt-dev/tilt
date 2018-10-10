package engine

import "sync"

// A central state store, modeled after the Reactive programming UX pattern.
// Terminology is borrowed liberally from Redux. These docs in particular are helpful:
// https://redux.js.org/introduction/threeprinciples
// https://redux.js.org/basics
type Store struct {
	state       *engineState
	actionQueue *actionQueue
	actionCh    chan Action
	mu          sync.Mutex

	// TODO(nick): Define Subscribers and Reducers.
	// The actionChan is an intermediate representation to make the transition easiser.
}

func NewStore(state *engineState) *Store {
	return &Store{
		state:       state,
		actionQueue: &actionQueue{},
		actionCh:    make(chan Action),
	}
}

// TODO(nick): Clone the state to ensure it's not mutated.
func (s *Store) State() engineState {
	return *(s.state)
}

func (s *Store) Actions() <-chan Action {
	return s.actionCh
}

func (s *Store) Dispatch(action Action) {
	s.actionQueue.add(action)
	go s.drainActions()
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
