package model

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type logEvent struct {
	ts      time.Time
	message string
}

func (l logEvent) Message() []byte {
	return []byte(l.message)
}

func (l logEvent) Time() time.Time {
	return l.ts
}

func TestLog_AppendUnderLimit(t *testing.T) {
	l := NewLog("foo")
	l = AppendLog(l, logEvent{time.Time{}, "bar"}, "", nil)
	assert.Equal(t, "foobar", l.String())
}

func TestLog_AppendOverLimit(t *testing.T) {
	l := NewLog("hello\n")
	sb := strings.Builder{}
	for i := 0; i < maxLogLengthInBytes/2; i++ {
		_, err := sb.WriteString("x\n")
		if err != nil {
			t.Fatalf("error in %T.WriteString: %+v", sb, err)
		}
	}

	s := sb.String()

	l = AppendLog(l, logEvent{time.Time{}, s}, "", nil)

	assert.Equal(t, s[:logTruncationTarget], l.String())
}

func TestLogPrefix(t *testing.T) {
	l := NewLog("hello\n")
	l = AppendLog(l, logEvent{time.Now(), "bar\nbaz\n"}, "prefix | ", nil)
	expected := "hello\nprefix | bar\nprefix | baz\n"
	assert.Equal(t, expected, l.String())
}

func TestScrubSecret(t *testing.T) {
	l := NewLog("")
	secretSet := SecretSet{}
	secretSet.AddSecret("my-secret", "client-id", []byte("secret"))
	l = AppendLog(l, logEvent{time.Now(), "hello\nsecret-time!\nc2VjcmV0-time!\ngoodbye"}, "", secretSet)
	assert.Equal(t, `hello
[redacted secret my-secret:client-id]-time!
[redacted secret my-secret:client-id]-time!
goodbye`, l.String())
}

func TestLogTail(t *testing.T) {
	l := NewLog("1\n2\n3\n4\n5\n")
	assert.Equal(t, "", l.Tail(0).String())
	assert.Equal(t, "5\n", l.Tail(1).String())
	assert.Equal(t, "4\n5\n", l.Tail(2).String())
	assert.Equal(t, "3\n4\n5\n", l.Tail(3).String())
	assert.Equal(t, "2\n3\n4\n5\n", l.Tail(4).String())
	assert.Equal(t, "1\n2\n3\n4\n5\n", l.Tail(5).String())
	assert.Equal(t, "1\n2\n3\n4\n5\n", l.Tail(6).String())
}
