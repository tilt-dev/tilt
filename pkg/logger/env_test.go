package logger

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrepareEnv(t *testing.T) {
	out := &bytes.Buffer{}
	ctx := WithLogger(context.Background(), NewTestLogger(out))

	tests := []struct {
		name     string
		env      []string
		expected []string
	}{
		{"defaults-nil", nil, []string{"LINES=24", "COLUMNS=80", "PYTHONUNBUFFERED=1"}},
		{"defaults", []string{}, []string{"LINES=24", "COLUMNS=80", "PYTHONUNBUFFERED=1"}},
		{"python-unbuffered", []string{"PYTHONUNBUFFERED=2"}, []string{"LINES=24", "COLUMNS=80", "PYTHONUNBUFFERED=2"}},
		{"wide-columns", []string{"COLUMNS=200"}, []string{"LINES=24", "COLUMNS=200", "PYTHONUNBUFFERED=1"}},
		{"so-many-lines", []string{"LINES=20000"}, []string{"LINES=20000", "COLUMNS=80", "PYTHONUNBUFFERED=1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.ElementsMatch(t, tt.expected, PrepareEnv(Get(ctx), tt.env))
		})
	}
}

func TestPrepareEnvWithColor(t *testing.T) {
	out := &bytes.Buffer{}
	logger := NewFuncLogger(true, DebugLvl, func(level Level, fields Fields, bytes []byte) error {
		_, err := out.Write(bytes)
		return err
	})
	ctx := WithLogger(context.Background(), logger)
	assert.ElementsMatch(t, []string{
		"FORCE_COLOR=1",
		"LINES=24",
		"COLUMNS=80",
		"PYTHONUNBUFFERED=1",
	}, PrepareEnv(Get(ctx), nil))
}
