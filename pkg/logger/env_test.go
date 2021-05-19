package logger

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultEnv(t *testing.T) {
	out := &bytes.Buffer{}
	ctx := WithLogger(context.Background(), NewLogger(DebugLvl, out))
	assert.Equal(t, []string{
		"LINES=24",
		"COLUMNS=80",
		"FORCE_COLOR=1",
		"PYTHONUNBUFFERED=1",
	}, PrepareEnv(Get(ctx), nil))
}

func TestPreservePythonUnbuffered(t *testing.T) {
	out := &bytes.Buffer{}
	ctx := WithLogger(context.Background(), NewLogger(DebugLvl, out))
	assert.Equal(t, []string{
		"PYTHONUNBUFFERED=",
		"LINES=24",
		"COLUMNS=80",
		"FORCE_COLOR=1",
	}, PrepareEnv(Get(ctx), []string{"PYTHONUNBUFFERED="}))
}
