package logstore

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/pkg/model"
)

type logEvent struct {
	name    model.ManifestName
	ts      time.Time
	message string
}

func newGlobalLogEvent(message string) logEvent {
	return newLogEvent("", time.Now(), message)
}

func newLogEvent(name model.ManifestName, ts time.Time, message string) logEvent {
	return logEvent{
		name:    name,
		ts:      ts,
		message: message,
	}
}

func (l logEvent) Message() []byte {
	return []byte(l.message)
}

func (l logEvent) Time() time.Time {
	return l.ts
}

func (l logEvent) Source() model.ManifestName {
	return l.name
}

func TestLog_AppendUnderLimit(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalLogEvent("foo"), nil)
	l.Append(newGlobalLogEvent("bar"), nil)
	assert.Equal(t, "foobar", l.String())
}

func TestLog_AppendOverLimit(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalLogEvent("hello\n"), nil)
	sb := strings.Builder{}
	for i := 0; i < maxLogLengthInBytes/2; i++ {
		_, err := sb.WriteString("x\n")
		if err != nil {
			t.Fatalf("error in %T.WriteString: %+v", sb, err)
		}
	}

	s := sb.String()
	l.Append(newGlobalLogEvent(s), nil)
	assert.Equal(t, s[:logTruncationTarget], l.String())
}

func TestLogPrefix(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalLogEvent("hello\n"), nil)
	l.Append(newLogEvent("prefix", time.Now(), "bar\nbaz\n"), nil)
	expected := "hello\nprefix | bar\nprefix | baz\n"
	assert.Equal(t, expected, l.String())
}

// Assert that when logs come from two different sources, they get interleaved correctly.
func TestLogInterleaving(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalLogEvent("hello ... "), nil)
	l.Append(newLogEvent("prefix", time.Now(), "START LONG MESSAGE\ngoodbye ... "), nil)
	l.Append(newGlobalLogEvent("world\nnext line of global log"), nil)
	l.Append(newLogEvent("prefix", time.Now(), "world\nEND LONG MESSAGE"), nil)
	expected := "hello ... world\nprefix | START LONG MESSAGE\nprefix | goodbye ... world\nnext line of global log\nprefix | END LONG MESSAGE"
	assert.Equal(t, expected, l.String())
}

func TestScrubSecret(t *testing.T) {
	l := NewLogStore()
	secretSet := model.SecretSet{}
	secretSet.AddSecret("my-secret", "client-id", []byte("secret"))
	l.Append(newGlobalLogEvent("hello\nsecret-time!\nc2VjcmV0-time!\ngoodbye"), secretSet)
	assert.Equal(t, `hello
[redacted secret my-secret:client-id]-time!
[redacted secret my-secret:client-id]-time!
goodbye`, l.String())
}

func TestLogTail(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalLogEvent("1\n2\n3\n4\n5\n"), nil)
	assert.Equal(t, "", l.Tail(0).String())
	assert.Equal(t, "5\n", l.Tail(1).String())
	assert.Equal(t, "4\n5\n", l.Tail(2).String())
	assert.Equal(t, "3\n4\n5\n", l.Tail(3).String())
	assert.Equal(t, "2\n3\n4\n5\n", l.Tail(4).String())
	assert.Equal(t, "1\n2\n3\n4\n5\n", l.Tail(5).String())
	assert.Equal(t, "1\n2\n3\n4\n5\n", l.Tail(6).String())
}

func TestLogTailPrefixes(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalLogEvent("1\n2\n"), nil)
	l.Append(newLogEvent("fe", time.Now(), "3\n4\n"), nil)
	l.Append(newGlobalLogEvent("5\n"), nil)
	assert.Equal(t, "", l.Tail(0).String())
	assert.Equal(t, "5\n", l.Tail(1).String())
	assert.Equal(t, "fe | 4\n5\n", l.Tail(2).String())
	assert.Equal(t, "fe | 3\nfe | 4\n5\n", l.Tail(3).String())
	assert.Equal(t, "2\nfe | 3\nfe | 4\n5\n", l.Tail(4).String())
	assert.Equal(t, "1\n2\nfe | 3\nfe | 4\n5\n", l.Tail(5).String())
	assert.Equal(t, "1\n2\nfe | 3\nfe | 4\n5\n", l.Tail(6).String())
}
