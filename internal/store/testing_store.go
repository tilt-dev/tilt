package store

import (
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"
)

var _ RStore = &TestingStore{}

type TestingStore struct {
	state   *EngineState
	stateMu sync.RWMutex

	actions   []Action
	actionsMu sync.RWMutex
}

func NewTestingStore() *TestingStore {
	return &TestingStore{
		state: NewState(),
	}
}

func (s *TestingStore) LockMutableStateForTesting() *EngineState {
	s.stateMu.Lock()
	return s.state
}

func (s *TestingStore) UnlockMutableState() {
	s.stateMu.Unlock()
}

func (s *TestingStore) SetState(state EngineState) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	s.state = &state
}

func (s *TestingStore) WithState(f func(state *EngineState)) {
	s.stateMu.Lock()
	defer s.stateMu.Unlock()
	f(s.state)
}

func (s *TestingStore) StateMutex() *sync.RWMutex {
	return &s.stateMu
}

func (s *TestingStore) RLockState() EngineState {
	s.stateMu.RLock()
	return *(s.state)
}

func (s *TestingStore) RUnlockState() {
	s.stateMu.RUnlock()
}

func (s *TestingStore) Dispatch(action Action) {
	s.actionsMu.Lock()
	defer s.actionsMu.Unlock()

	s.actions = append(s.actions, action)
}

func (s *TestingStore) ClearActions() {
	s.actionsMu.Lock()
	defer s.actionsMu.Unlock()

	s.actions = nil
}

func (s *TestingStore) Actions() []Action {
	s.actionsMu.RLock()
	defer s.actionsMu.RUnlock()

	return append([]Action{}, s.actions...)
}

func (s *TestingStore) AssertNoErrorActions(t testing.TB) {
	t.Helper()
	for _, action := range s.actions {
		errAction, ok := action.(ErrorAction)
		if ok {
			t.Errorf("Error action: %s", errAction)
		}
	}
}

// for use by tests (with a real channel-based store, NOT a TestingStore), to wait until
// an action of the specified type comes out of the given chan at some point we might want
// it to return the index it was found at, and then take an index, so that we can start
// searching from the next index
func WaitForAction(t testing.TB, typ reflect.Type, getActions func() []Action) Action {
	start := time.Now()
	timeout := 500 * time.Millisecond

	for time.Since(start) < timeout {
		actions := getActions()
		for _, a := range actions {
			if reflect.TypeOf(a) == typ {
				return a
			} else if la, ok := a.(LogAction); ok {
				fmt.Println(string(la.Message()))
			}
		}
	}
	t.Fatalf("timed out waiting for action of type %s. Saw the following actions while waiting: %+v",
		typ.Name(),
		getActions())
	return nil
}

// for use by tests (with a real channel-based store, NOT a TestingStore). Assert that
// we don't see an action of the given type
func AssertNoActionOfType(t testing.TB, typ reflect.Type, getActions func() []Action) Action {
	start := time.Now()
	timeout := 300 * time.Millisecond

	for time.Since(start) < timeout {
		actions := getActions()
		for _, a := range actions {
			if reflect.TypeOf(a) == typ {
				t.Fatalf("Found action of type %s where none was expected: %+v", typ.Name(), a)
			} else if la, ok := a.(LogAction); ok {
				fmt.Println(string(la.Message()))
			}
		}
	}
	return nil
}
