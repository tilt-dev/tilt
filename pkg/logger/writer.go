package logger

import (
	"io"
	"sync"
)

type MutexWriter struct {
	underlying io.Writer
	mu         *sync.Mutex
}

func NewMutexWriter(underlying io.Writer) MutexWriter {
	return MutexWriter{
		underlying: underlying,
		mu:         &sync.Mutex{},
	}
}

func (w MutexWriter) Write(b []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.underlying.Write(b)
}
