package print

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/pkg/logger"
)

func TestWarn(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File("Tiltfile", "warn('problem 1')")
	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	assert.Contains(t, f.PrintOutput(), "problem 1")
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

func TestExit(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File(
		"Tiltfile", `
exit("goodbye")
fail("this can't happen!")
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	out := f.PrintOutput()
	assert.Contains(t, out, "goodbye")
	assert.NotContains(t, out, "this can't happen!")
}

func TestExitNoMessage(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File(
		"Tiltfile", `
exit()
fail("this can't happen!")
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	assert.Empty(t, f.PrintOutput())
}

func newFixture(tb testing.TB) *starkit.Fixture {
	f := starkit.NewFixture(tb, NewExtension())
	out := bytes.NewBuffer(nil)
	f.SetOutput(out)
	log := logger.NewLogger(logger.VerboseLvl, out)
	ctx := logger.WithLogger(context.Background(), log)
	f.SetContext(ctx)
	return f
}
