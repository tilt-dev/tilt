package logstore

import "sync"

// Thread-safe reading a log store, outside of the Store state loop.
type Reader struct {
	mu    *sync.RWMutex
	store *LogStore
}

func NewReader(mu *sync.RWMutex, store *LogStore) Reader {
	return Reader{mu: mu, store: store}
}

func (r Reader) Checkpoint() Checkpoint {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.store.Checkpoint()
}

func (r Reader) Empty() bool {
	if r.store == nil {
		return true
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.store.Empty()
}

func (r Reader) String() string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.store.String()
}

func (r Reader) ContinuingString(c Checkpoint) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.store.ContinuingString(c)
}

func (r Reader) Tail(n int) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.store.Tail(n)
}

func (r Reader) TailSpan(n int, spanID SpanID) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.store.TailSpan(n, spanID)
}
