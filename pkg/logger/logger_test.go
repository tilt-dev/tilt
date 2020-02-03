package logger

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCtxWithForkedOutput(t *testing.T) {
	out1 := &bytes.Buffer{}
	out2 := &bytes.Buffer{}

	ctx := WithLogger(context.Background(), NewLogger(DebugLvl, out1))
	l := Get(CtxWithForkedOutput(ctx, out2))

	l.Infof("test %s", "abcd")
	l.Debugf("test2 %d", 5)

	assert.Equal(t, "test abcd\ntest2 5\n", out1.String())
	assert.Equal(t, "test abcd\ntest2 5\n", out2.String())
}

func TestWriteAcrossNestedLoggers(t *testing.T) {
	out1 := bytes.NewBuffer(nil)
	out2 := bytes.NewBuffer(nil)
	prefixedOut1 := NewPrefixedLogger("|", NewLogger(DebugLvl, out1))
	ctx := WithLogger(context.Background(), prefixedOut1)
	l := Get(CtxWithForkedOutput(ctx, out2))
	w := NewPrefixedLogger(">", l).Writer(InfoLvl)

	_, _ = w.Write([]byte("a"))
	_, _ = w.Write([]byte("b\nc"))
	_, _ = w.Write([]byte("d\ne\n"))

	assert.Equal(t, "|>ab\n|>cd\n|>e\n", out1.String())
	assert.Equal(t, ">ab\n>cd\n>e\n", out2.String())
}

func TestWriteWithFormatPlaceholder(t *testing.T) {
	out := bytes.NewBuffer(nil)
	l := NewLogger(InfoLvl, out)
	l.Write(InfoLvl, []byte("%s"))
	assert.Equal(t, "%s", out.String())
}
