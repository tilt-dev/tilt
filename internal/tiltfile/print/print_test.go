package print

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/pkg/logger"
)

func TestWarn(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	out := bytes.NewBuffer(nil)
	log := logger.NewLogger(logger.WarnLvl, out)
	ctx := logger.WithLogger(context.Background(), log)
	f.SetContext(ctx)

	f.File("Tiltfile", "warn('problem 1')")
	_, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.Equal(t, "problem 1\n", out.String())
}

func TestFail(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()
	f.File("Tiltfile", "fail('problem 1')")
	_, err := f.ExecFile("Tiltfile")
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "problem 1")
	}
}

func newFixture(tb testing.TB) *starkit.Fixture {
	return starkit.NewFixture(tb, NewExtension())
}
