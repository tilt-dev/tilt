// todo: compile only in test mode

package store

func StoreWithState(state *EngineState) *Store {
	newStore := NewStore()
	newStore.state = state
	return newStore
}
