package logstore

import (
	"fmt"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

// NOTE(dmiller): set at runtime with:
// go test -ldflags="-X 'github.com/windmilleng/tilt/pkg/model/logstore.LogstoreWriteGoldenMaster=1'" ./pkg/model/logstore
var LogstoreWriteGoldenMaster = "0"

func TestLog_AppendUnderLimit(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalTestLogEvent("foo"), nil)
	l.Append(newGlobalTestLogEvent("bar"), nil)
	assert.Equal(t, "foobar", l.String())
}

func TestAppendDifferentLevels(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalLevelTestLogEvent("foo", logger.InfoLvl), nil)
	l.Append(newGlobalLevelTestLogEvent("bar", logger.DebugLvl), nil)
	l.Append(newGlobalLevelTestLogEvent("baz", logger.InfoLvl), nil)
	assert.Equal(t, "foo\nbar\nbaz", l.String())
}

func TestAppendDifferentLevelsMultiLines(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalTestLogEvent("hello ... "), nil)
	l.Append(newGlobalLevelTestLogEvent("foobar", logger.DebugLvl), nil)
	l.Append(newGlobalTestLogEvent("world\nnext line of global log"), nil)
	assert.Equal(t, "hello ... \nfoobar\nworld\nnext line of global log", l.String())

	l.recomputeDerivedValues()
	assert.Equal(t, "hello ... \nfoobar\nworld\nnext line of global log", l.String())
}

func TestLog_AppendOverLimit(t *testing.T) {
	l := NewLogStore()
	l.maxLogLengthInBytes = 100

	l.Append(newGlobalTestLogEvent("hello\n"), nil)
	sb := strings.Builder{}
	for i := 0; i < l.maxLogLengthInBytes/2; i++ {
		_, err := sb.WriteString("x\n")
		if err != nil {
			t.Fatalf("error in %T.WriteString: %+v", sb, err)
		}
	}

	s := sb.String()
	l.Append(newGlobalTestLogEvent(s), nil)
	assert.Equal(t, s[:l.logTruncationTarget()], l.String())
}

func TestLogPrefix(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalTestLogEvent("hello\n"), nil)
	l.Append(newTestLogEvent("prefix", time.Now(), "bar\nbaz\n"), nil)

	assertSnapshot(t, l.String())
}

// Assert that when logs come from two different sources, they get interleaved correctly.
func TestLogInterleaving(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalTestLogEvent("hello ... "), nil)
	l.Append(newTestLogEvent("prefix", time.Now(), "START LONG MESSAGE\ngoodbye ... "), nil)
	l.Append(newGlobalTestLogEvent("world\nnext line of global log"), nil)
	l.Append(newTestLogEvent("prefix", time.Now(), "world\nEND LONG MESSAGE"), nil)

	assertSnapshot(t, l.String())
}

func TestScrubSecret(t *testing.T) {
	l := NewLogStore()
	secretSet := model.SecretSet{}
	secretSet.AddSecret("my-secret", "client-id", []byte("secret"))
	l.Append(newGlobalTestLogEvent("hello\nsecret-time!\nc2VjcmV0-time!\ngoodbye"), secretSet)
	assert.Equal(t, `hello
[redacted secret my-secret:client-id]-time!
[redacted secret my-secret:client-id]-time!
goodbye`, l.String())
}

func TestLogTail(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalTestLogEvent("1\n2\n3\n4\n5\n"), nil)
	assert.Equal(t, "", l.Tail(0))
	assert.Equal(t, "5\n", l.Tail(1))
	assert.Equal(t, "4\n5\n", l.Tail(2))
	assert.Equal(t, "3\n4\n5\n", l.Tail(3))
	assert.Equal(t, "2\n3\n4\n5\n", l.Tail(4))
	assert.Equal(t, "1\n2\n3\n4\n5\n", l.Tail(5))
	assert.Equal(t, "1\n2\n3\n4\n5\n", l.Tail(6))
}

func TestLogTailPrefixes(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalTestLogEvent("1\n2\n"), nil)
	l.Append(newTestLogEvent("fe", time.Now(), "3\n4\n"), nil)
	l.Append(newGlobalTestLogEvent("5\n"), nil)
	assert.Equal(t, "", l.Tail(0))
	assert.Equal(t, "5\n", l.Tail(1))
	assert.Equal(t, "           fe │ 4\n5\n", l.Tail(2))
	assert.Equal(t, "           fe │ 3\n           fe │ 4\n5\n", l.Tail(3))
	assert.Equal(t, "2\n           fe │ 3\n           fe │ 4\n5\n", l.Tail(4))
	assert.Equal(t, "1\n2\n           fe │ 3\n           fe │ 4\n5\n", l.Tail(5))
	assert.Equal(t, "1\n2\n           fe │ 3\n           fe │ 4\n5\n", l.Tail(6))
}

func TestLogTailSpan(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalTestLogEvent("1\n2\n"), nil)
	l.Append(newTestLogEvent("fe", time.Now(), "3\n4\n"), nil)
	l.Append(newGlobalTestLogEvent("5\n"), nil)
	assert.Equal(t, "5\n", l.TailSpan(1, ""))
	assert.Equal(t, "2\n5\n", l.TailSpan(2, ""))
	assert.Equal(t, "1\n2\n5\n", l.TailSpan(3, ""))
	assert.Equal(t, "4\n", l.TailSpan(1, "fe"))
	assert.Equal(t, "3\n4\n", l.TailSpan(2, "fe"))
	assert.Equal(t, "3\n4\n", l.TailSpan(3, "fe"))
	assert.Equal(t, "3\n4\n", l.TailSpan(30, "fe"))
}

func TestLogTailParts(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalTestLogEvent("a"), nil)
	l.Append(newTestLogEvent("fe", time.Now(), "xy"), nil)
	l.Append(newGlobalTestLogEvent("bc\n"), nil)
	l.Append(newTestLogEvent("fe", time.Now(), "z\n"), nil)
	assert.Equal(t, "           fe │ xyz\n", l.Tail(1))
	assert.Equal(t, "abc\n           fe │ xyz\n", l.Tail(2))
}

func TestContinuingString(t *testing.T) {
	l := NewLogStore()

	c1 := l.Checkpoint()
	assert.Equal(t, "", l.ContinuingString(c1))

	l.Append(newGlobalTestLogEvent("foo"), nil)
	c2 := l.Checkpoint()
	assert.Equal(t, "foo", l.ContinuingString(c1))

	l.Append(newGlobalTestLogEvent("bar\n"), nil)
	assert.Equal(t, "foobar\n", l.ContinuingString(c1))
	assert.Equal(t, "bar\n", l.ContinuingString(c2))
}

func TestContinuingStringOneSource(t *testing.T) {
	l := NewLogStore()

	c1 := l.Checkpoint()
	assert.Equal(t, "", l.ContinuingString(c1))

	l.Append(newTestLogEvent("fe", time.Now(), "foo"), nil)
	c2 := l.Checkpoint()
	assert.Equal(t, "           fe │ foo", l.ContinuingString(c1))

	l.Append(newTestLogEvent("fe", time.Now(), "bar\n"), nil)
	assert.Equal(t, "           fe │ foobar\n", l.ContinuingString(c1))
	assert.Equal(t, "bar\n", l.ContinuingString(c2))
}

func TestContinuingStringTwoSources(t *testing.T) {
	l := NewLogStore()

	c1 := l.Checkpoint()

	l.Append(newGlobalTestLogEvent("a"), nil)
	c2 := l.Checkpoint()
	assert.Equal(t, "a", l.ContinuingString(c1))

	l.Append(newTestLogEvent("fe", time.Now(), "xy"), nil)
	c3 := l.Checkpoint()
	assert.Equal(t, "a\n           fe │ xy", l.ContinuingString(c1))
	assert.Equal(t, "\n           fe │ xy", l.ContinuingString(c2))

	l.Append(newGlobalTestLogEvent("bc\n"), nil)
	c4 := l.Checkpoint()
	assert.Equal(t, "abc\n           fe │ xy", l.ContinuingString(c1))
	assert.Equal(t, "\n           fe │ xy\nbc\n", l.ContinuingString(c2))
	assert.Equal(t, "\nbc\n", l.ContinuingString(c3))

	l.Append(newTestLogEvent("fe", time.Now(), "z\n"), nil)
	assert.Equal(t, "abc\n           fe │ xyz\n", l.ContinuingString(c1))
	assert.Equal(t, "\n           fe │ xyz\nbc\n", l.ContinuingString(c2))
	assert.Equal(t, "\nbc\n           fe │ z\n", l.ContinuingString(c3))
	assert.Equal(t, "           fe │ z\n", l.ContinuingString(c4))
}

func TestContinuingStringAfterLimit(t *testing.T) {
	l := NewLogStore()
	l.maxLogLengthInBytes = 20

	c1 := l.Checkpoint()
	assert.Equal(t, "", l.ContinuingString(c1))

	l.Append(newGlobalTestLogEvent("123456789\n"), nil)
	c2 := l.Checkpoint()
	assert.Equal(t, "123456789\n", l.String())
	assert.Equal(t, "123456789\n", l.ContinuingString(c1))

	l.Append(newGlobalTestLogEvent("abcdefghi\n"), nil)
	c3 := l.Checkpoint()
	assert.Equal(t, "123456789\nabcdefghi\n", l.String())
	assert.Equal(t, "123456789\nabcdefghi\n", l.ContinuingString(c1))
	assert.Equal(t, "abcdefghi\n", l.ContinuingString(c2))

	l.Append(newGlobalTestLogEvent("jklmnopqr\n"), nil)
	assert.Equal(t, "jklmnopqr\n", l.String())
	assert.Equal(t, "jklmnopqr\n", l.ContinuingString(c1))
	assert.Equal(t, "jklmnopqr\n", l.ContinuingString(c2))
	assert.Equal(t, "jklmnopqr\n", l.ContinuingString(c3))
}

func TestManifestLog(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalTestLogEvent("1\n2\n"), nil)
	l.Append(newTestLogEvent("fe", time.Now(), "3\n4\n"), nil)
	l.Append(newGlobalTestLogEvent("5\n6\n"), nil)
	l.Append(newTestLogEvent("fe", time.Now(), "7\n8\n"), nil)
	l.Append(newTestLogEvent("back", time.Now(), "a\nb\n"), nil)
	l.Append(newGlobalTestLogEvent("5\n6\n"), nil)
	assert.Equal(t, "3\n4\n7\n8\n", l.ManifestLog("fe"))
	assert.Equal(t, "a\nb\n", l.ManifestLog("back"))
}

func TestManifestLogContinuation(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalTestLogEvent("1\n2\n"), nil)
	l.Append(newTestLogEvent("fe", time.Now(), "34"), nil)
	l.Append(newGlobalTestLogEvent("5\n6\n"), nil)
	l.Append(newTestLogEvent("fe", time.Now(), "78"), nil)
	l.Append(newTestLogEvent("back", time.Now(), "ab"), nil)
	l.Append(newGlobalTestLogEvent("5\n6\n"), nil)
	assert.Equal(t, "3478", l.ManifestLog("fe"))
	assert.Equal(t, "ab", l.ManifestLog("back"))
	assert.Equal(t, "1\n2\n           fe │ 3478\n5\n6\n         back │ ab\n5\n6\n", l.String())
}

func TestLogIncremental(t *testing.T) {
	l := NewLogStore()
	l.Append(newGlobalTestLogEvent("line1\n"), nil)
	l.Append(newGlobalTestLogEvent("line2\n"), nil)
	l.Append(newGlobalTestLogEvent("line3\n"), nil)

	list, err := l.ToLogList(0)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(list.Segments))
	assert.Equal(t, int32(0), list.FromCheckpoint)
	assert.Equal(t, int32(3), list.ToCheckpoint)

	list, err = l.ToLogList(1)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(list.Segments))
	assert.Equal(t, int32(1), list.FromCheckpoint)
	assert.Equal(t, int32(3), list.ToCheckpoint)

	list, err = l.ToLogList(3)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(list.Segments))
	assert.Equal(t, int32(-1), list.FromCheckpoint)
	assert.Equal(t, int32(-1), list.ToCheckpoint)

	list, err = l.ToLogList(10)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(list.Segments))
	assert.Equal(t, int32(-1), list.FromCheckpoint)
	assert.Equal(t, int32(-1), list.ToCheckpoint)
}

func TestWarnings(t *testing.T) {
	l := NewLogStore()
	l.Append(testLogEvent{
		name:    "fe",
		level:   logger.WarnLvl,
		message: "Warning 1 line 1\nWarning 1 line 2\nWarning 1 line 3\n",
	}, nil)
	l.Append(testLogEvent{
		name:    "fe",
		level:   logger.WarnLvl,
		message: "Warning 2 line 1\nWarning 2 line 2\n",
	}, nil)
	l.Append(testLogEvent{
		name:    "fe",
		level:   logger.WarnLvl,
		message: "Warning 3 line 1\n",
	}, nil)
	l.Append(testLogEvent{
		name:    "non-fe",
		level:   logger.WarnLvl,
		message: "non-fe warning\n",
	}, nil)

	warnings := l.Warnings("fe")
	assert.Equal(t, warnings, []string{
		"Warning 1 line 1\nWarning 1 line 2\nWarning 1 line 3\n",
		"Warning 2 line 1\nWarning 2 line 2\n",
		"Warning 3 line 1\n",
	})

	assertSnapshot(t, l.String())
}

func TestErrors(t *testing.T) {
	l := NewLogStore()
	l.Append(testLogEvent{
		name:    "fe",
		level:   logger.ErrorLvl,
		message: "Error 1 line 1\nError 1 line 2\nError 1 line 3\n",
	}, nil)
	l.Append(testLogEvent{
		name:    "fe",
		level:   logger.ErrorLvl,
		message: "Error 2 line 1\nError 2 line 2\n",
	}, nil)
	l.Append(testLogEvent{
		name:    "fe",
		level:   logger.ErrorLvl,
		message: "Error 3 line 1\n",
	}, nil)
	l.Append(testLogEvent{
		name:    "non-fe",
		level:   logger.ErrorLvl,
		message: "non-fe warning\n",
	}, nil)

	assertSnapshot(t, l.String())
}

func TestContinuingLines(t *testing.T) {
	l := NewLogStore()
	c1 := l.Checkpoint()

	now := time.Now()
	l.Append(testLogEvent{
		name:    "fe",
		message: "layer 1: pending\n",
		ts:      now,
		fields:  map[string]string{logger.FieldNameProgressID: "layer 1"},
	}, nil)
	l.Append(testLogEvent{
		name:    "fe",
		message: "layer 2: pending\n",
		ts:      now,
		fields:  map[string]string{logger.FieldNameProgressID: "layer 2"},
	}, nil)

	assert.Equal(t, "           fe │ layer 1: pending\n           fe │ layer 2: pending\n",
		l.ContinuingString(c1))

	c2 := l.Checkpoint()
	assert.Equal(t, []LogLine{
		LogLine{Text: "           fe │ layer 1: pending\n", SpanID: "fe", ProgressID: "layer 1", Time: now},
		LogLine{Text: "           fe │ layer 2: pending\n", SpanID: "fe", ProgressID: "layer 2", Time: now},
	}, l.ContinuingLines(c1))

	l.Append(testLogEvent{
		name:    "fe",
		message: "layer 1: done\n",
		ts:      now,
		fields: map[string]string{
			logger.FieldNameProgressID:        "layer 1",
			logger.FieldNameProgressMustPrint: "1",
		},
	}, nil)

	assert.Equal(t, []LogLine{
		LogLine{
			Text:              "           fe │ layer 1: done\n",
			SpanID:            "fe",
			ProgressID:        "layer 1",
			ProgressMustPrint: true,
			Time:              now,
		},
	}, l.ContinuingLines(c2))
}

func TestBuildEventInit(t *testing.T) {
	l := NewLogStore()

	now := time.Now()
	l.Append(testLogEvent{
		name:    "",
		message: "starting tilt\n",
		ts:      now,
	}, nil)
	l.Append(testLogEvent{
		name:    "fe",
		message: "init fe build\n",
		ts:      now,
		fields:  map[string]string{logger.FieldNameBuildEvent: "init"},
	}, nil)
	l.Append(testLogEvent{
		name:    "db",
		message: "init db build\n",
		ts:      now,
		fields:  map[string]string{logger.FieldNameBuildEvent: "init"},
	}, nil)

	assert.Equal(t, 5, len(l.toLogLines(logOptions{spans: l.spans})))

	assertSnapshot(t, l.String())
}

func assertSnapshot(t *testing.T, output string) {
	d1 := []byte(output)
	gmPath := fmt.Sprintf("testdata/%s_master", t.Name())
	if LogstoreWriteGoldenMaster == "1" {
		err := ioutil.WriteFile(gmPath, d1, 0644)
		if err != nil {
			t.Fatal(err)
		}
	}
	expected, err := ioutil.ReadFile(gmPath)
	if err != nil {
		t.Fatal(err)
	}

	if string(expected) != output {
		t.Errorf("Expected: %s != Output: %s", expected, output)
	}
}
