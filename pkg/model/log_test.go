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
	l = AppendLog(l, logEvent{time.Time{}, "bar"}, false, "")
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

	l = AppendLog(l, logEvent{time.Time{}, s}, false, "")

	assert.Equal(t, s, l.String())
}

func TestLog_Timestamps(t *testing.T) {
	// initial text ends with a newline - we want to ensure that we insert a timestamp when appending right after a newline
	l := NewLog("hello\n")

	ts, err := time.Parse(time.RFC3339, "2019-03-06T12:34:56Z")
	if err != nil {
		t.Fatal(err)
	}

	// appended text has a newline in the middle of the text (which should get a timestamp)
	// and at the end of the text (which shouldn't)
	l = AppendLog(l, logEvent{ts, "bar\nbaz\n"}, true, "")

	expected := "hello\n2019/03/06 12:34:56 bar\n2019/03/06 12:34:56 baz\n"
	assert.Equal(t, expected, l.String())
}

func TestLogPrefix(t *testing.T) {
	l := NewLog("hello\n")
	l = AppendLog(l, logEvent{time.Now(), "bar\nbaz\n"}, false, "prefix | ")
	expected := "hello\nprefix | bar\nprefix | baz\n"
	assert.Equal(t, expected, l.String())
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
