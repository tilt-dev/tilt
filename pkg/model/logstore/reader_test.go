package logstore

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReader(t *testing.T) {
	l := NewLogStore()
	r := NewReader(&sync.RWMutex{}, l)

	c1 := r.Checkpoint()
	assert.Equal(t, "", r.ContinuingString(c1))

	l.Append(newGlobalTestLogEvent("foo"), nil)
	c2 := r.Checkpoint()
	assert.Equal(t, "foo", r.ContinuingString(c1))

	l.Append(newGlobalTestLogEvent("bar\n"), nil)
	_ = c2
	assert.Equal(t, "foobar\n", r.String())
	assert.Equal(t, "foobar\n", r.ContinuingString(c1))
	assert.Equal(t, "bar\n", r.ContinuingString(c2))

	l.Append(newGlobalTestLogEvent("abc\n"), nil)
	assert.Equal(t, "abc\n", l.Tail(1))
	assert.Equal(t, "abc\n", r.Tail(1))
}
