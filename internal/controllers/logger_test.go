package controllers

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/pkg/logger"
)

func TestLogrFacade_Info(t *testing.T) {
	var b bytes.Buffer
	tiltLogger := logger.NewLogger(logger.DebugLvl, &b)
	l := newLogrFacade(tiltLogger, "")
	l.Info("log message", "key", errors.New("surprise"))
	assert.Equal(t, "log message key=surprise\n", b.String())
}

func TestLogrFacade_Error(t *testing.T) {
	var b bytes.Buffer
	tiltLogger := logger.NewLogger(logger.DebugLvl, &b)
	l := newLogrFacade(tiltLogger, "")
	l.Error(errors.New("fake error"), "message", "extra", 1234)
	assert.Equal(t, "message error=\"fake error\" extra=1234\n", b.String())
}

func TestLogrFacade_Options(t *testing.T) {
	var b bytes.Buffer
	tiltLogger := logger.NewLogger(logger.DebugLvl, &b)
	l := newLogrFacade(tiltLogger, "parent").
		WithName("child").
		WithValues("key1", true, "key2", 3.141).
		V(100)
	// N.B. there's intentionally no corresponding value for key3
	l.Error(errors.New("oops"), "hi", "key3")
	assert.Equal(t, "hi logger=parent.child v=100 key1=true key2=3.141 error=oops key3=\n", b.String())
}
