package print

import (
	"bytes"
	"context"
	"fmt"
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

func TestExitArgTypes(t *testing.T) {
	type tc struct {
		name        string
		exitArg     string
		expectedLog string
	}

	tcs := []tc{
		{"Omitted", ``, ""},
		{"String", `"goodbye"`, "goodbye"},
		{"StringNamed", `code='ciao'`, "ciao"},
		{"Int", `123`, "123"},
		{"Dict", `dict(foo='bar', baz=123)`, `{"foo": "bar", "baz": 123}`},
		{"None", `None`, ""},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			f := newFixture(t)
			defer f.TearDown()

			f.File(
				"Tiltfile", fmt.Sprintf(`
exit(%s)
fail("this can't happen!")
`, tc.exitArg))

			_, err := f.ExecFile("Tiltfile")
			require.NoError(t, err)
			out := f.PrintOutput()
			if tc.expectedLog == "" {
				assert.Empty(t, out)
			} else {
				assert.Contains(t, out, tc.expectedLog)
				assert.NotContains(t, out, "this can't happen!")
			}
		})
	}
}

func TestExitLoadedTiltfile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.File("exit.tiltfile", `exit("later alligator")`)

	// loaded Tiltfile can force the root Tiltfile to halt execution
	// i.e. it's more like `sys.exit(0)` than `return`
	f.File(
		"Tiltfile", `
load("./exit.tiltfile", "this_symbol_does_not_exist")
fail("this can't happen!")
`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)
	out := f.PrintOutput()
	assert.Contains(t, out, "later alligator")
	assert.NotContains(t, out, "this can't happen!")
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
