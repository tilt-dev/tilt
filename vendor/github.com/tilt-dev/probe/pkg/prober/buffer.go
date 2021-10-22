package prober

import (
	"bytes"
	"sync"
)

type threadSafeBuffer struct {
	buf bytes.Buffer
	mu  sync.Mutex
}

func (b *threadSafeBuffer) Write(p []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *threadSafeBuffer) Bytes() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Bytes()
}
