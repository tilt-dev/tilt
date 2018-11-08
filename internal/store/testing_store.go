package store

import "sync"

type TestingStore struct {
	state   *EngineState
	stateMu sync.RWMutex

	Actions []Action
}

func NewTestingStore() *TestingStore {
	return &TestingStore{}
}

func (s *TestingStore) SetState(state EngineState) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.state = &state
}

func (s *TestingStore) RLockState() EngineState {
	s.stateMu.RLock()
	return *(s.state)
}

func (s *TestingStore) RUnlockState() {
	s.stateMu.RUnlock()
}

func (s *TestingStore) Dispatch(action Action) {
	s.Actions = append(s.Actions, action)
}
