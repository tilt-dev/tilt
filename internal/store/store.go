package store

import (
	"context"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"gopkg.in/d4l3k/messagediff.v1"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
)

// Allow actions to batch together a bit.
const actionBatchWindow = time.Millisecond

// Read-only store
type RStore interface {
	Dispatch(action Action)
	RLockState() EngineState
	RUnlockState()
}

// A central state store, modeled after the Reactive programming UX pattern.
// Terminology is borrowed liberally from Redux. These docs in particular are helpful:
// https://redux.js.org/introduction/threeprinciples
// https://redux.js.org/basics
type Store struct {
	state       *EngineState
	subscribers *subscriberList
	actionQueue *actionQueue
	actionCh    chan []Action
	mu          sync.Mutex
	stateMu     sync.RWMutex
	reduce      Reducer
	logActions  bool

	// TODO(nick): Define Subscribers and Reducers.
	// The actionChan is an intermediate representation to make the transition easiser.
}

func NewStore(reducer Reducer, logActions LogActionsFlag) *Store {
	return &Store{
		state:       NewState(),
		reduce:      reducer,
		actionQueue: &actionQueue{},
		actionCh:    make(chan []Action),
		subscribers: &subscriberList{},
		logActions:  bool(logActions),
	}
}

func NewStoreForTesting() *Store {
	return NewStore(EmptyReducer, false)
}

func (s *Store) AddSubscriber(sub Subscriber) {
	s.subscribers.Add(sub)
}

func (s *Store) RemoveSubscriber(sub Subscriber) error {
	return s.subscribers.Remove(sub)
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

		case actions := <-s.actionCh:
			s.stateMu.Lock()

			for _, action := range actions {
				var oldState EngineState
				if s.logActions {
					oldState = s.cheapCopyState()
				}

				s.reduce(ctx, s.state, action)

				if s.logActions {
					newState := s.cheapCopyState()
					go func() {
						diff, equal := messagediff.PrettyDiff(oldState, newState)
						if !equal {
							logger.Get(ctx).Infof("action %T:\n%s\ncaused state change:\n%s\n", action, spew.Sdump(action), diff)
						}
					}()
				}
			}

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

	if state.PermanentError != nil {
		return true, state.PermanentError
	}

	if state.UserExited {
		return true, nil
	}

	if len(state.ManifestTargets) == 0 {
		return false, nil
	}

	finished := !state.WatchMounts &&
		state.CompletedBuildCount == state.InitialBuildCount
	return finished, nil
}

func (s *Store) drainActions() {
	time.Sleep(actionBatchWindow)

	// The mutex here ensures that the actions appear on the channel in-order.
	// Otherwise, two drains can interleave badly.
	s.mu.Lock()
	defer s.mu.Unlock()

	actions := s.actionQueue.drain()
	if len(actions) > 0 {
		s.actionCh <- actions
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

type LogActionsFlag bool

// This does a partial deep copy for the purposes of comparison
// i.e., it ensures fields that will be useful in action logging get copied
// some fields might not be copied and might still point to the same instance as s.state
// and thus might reflect changes that happened as part of the current action or any future action
func (s *Store) cheapCopyState() EngineState {
	ret := *s.state
	targets := ret.ManifestTargets
	ret.ManifestTargets = make(map[model.ManifestName]*ManifestTarget)
	for k, v := range targets {
		ms := *(v.State)
		target := &ManifestTarget{
			Manifest: v.Manifest,
			State:    &ms,
		}

		ret.ManifestTargets[k] = target
	}
	return ret
}
