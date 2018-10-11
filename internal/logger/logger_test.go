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

	assert.Equal(t, out1.String(), out2.String())
}
