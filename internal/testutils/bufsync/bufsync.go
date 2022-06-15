package bufsync

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

func (b *ThreadSafeBuffer) AssertEventuallyContains(tb testing.TB, expected string, timeout time.Duration) {
	tb.Helper()
	assert.Eventuallyf(tb, func() bool {
		return strings.Contains(b.String(), expected)
	}, timeout, 10*time.Millisecond,
		"Expected: %q. Actual: %q", expected, LazyString(b.String))
}

func (b *ThreadSafeBuffer) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.buf.Reset()
}

var _ io.Writer = &ThreadSafeBuffer{}

type LazyString func() string

func (s LazyString) String() string {
	return s()
}
