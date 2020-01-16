package bufsync

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

type ThreadSafeBuffer struct {
	buf *bytes.Buffer
	mu  sync.Mutex
}

func NewThreadSafeBuffer() *ThreadSafeBuffer {
	return &ThreadSafeBuffer{
		buf: bytes.NewBuffer(nil),
	}
}

func (b *ThreadSafeBuffer) Write(bs []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(bs)
}

func (b *ThreadSafeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *ThreadSafeBuffer) WaitUntilContains(expected string, timeout time.Duration) error {
	start := time.Now()
	for time.Since(start) < timeout {
		result := b.String()
		if strings.Contains(result, expected) {
			return nil
		}
		time.Sleep(10 * time.Millisecond)
	}

	return fmt.Errorf("Timeout. Expected %q. Actual: %s", expected, b.String())
}

func (b *ThreadSafeBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.buf.Reset()
}

var _ io.Writer = &ThreadSafeBuffer{}
