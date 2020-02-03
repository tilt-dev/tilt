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
	if r.store == nil {
		return 0
	}

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
	if r.store == nil {
		return ""
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.store.String()
}

func (r Reader) ContinuingString(c Checkpoint) string {
	if r.store == nil {
		return ""
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.store.ContinuingString(c)
}

func (r Reader) ContinuingLines(c Checkpoint) []LogLine {
	if r.store == nil {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.store.ContinuingLines(c)
}

func (r Reader) Tail(n int) string {
	if r.store == nil {
		return ""
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.store.Tail(n)
}

func (r Reader) TailSpan(n int, spanID SpanID) string {
	if r.store == nil {
		return ""
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.store.TailSpan(n, spanID)
}

func (r Reader) Warnings(spanID SpanID) []string {
	if r.store == nil {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.store.Warnings(spanID)
}
