package value

import (
	"encoding/json"
	"fmt"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
)

func TestCmdPlugin(t *testing.T) {
	type tc struct {
		input string
		args  interface{}
		dir   string
		env   []string
	}

	tcs := []tc{
		{input: `cmd('echo hi')`, args: "echo hi"},
		{input: `cmd(['echo', 'hi'])`, args: []string{"echo", "hi"}},
		{
			input: `cmd(args='cat foo.txt', env={'FOO': 'bar', 'bar': 'FOO'})`,
			args:  "cat foo.txt",
			env:   []string{"FOO=bar", "bar=FOO"},
		},
		{
			input: `cmd(args=["abc"], dir='/foo/bar')`,
			args:  []string{"abc"},
			// N.B. there's no OS validation of this path, so it's fine that this isn't portable for Windows
			dir: "/foo/bar",
		},
		{
			input: `cmd(args=["abc"], env={"A": "1"}, dir='/foo/bar')`,
			args:  []string{"abc"},
			// N.B. there's no OS validation of this path, so it's fine that this isn't portable for Windows
			dir: "/foo/bar",
			env: []string{"A=1"},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.input, func(t *testing.T) {
			f := starkit.NewFixture(t, NewCmdPlugin())
			defer f.TearDown()

			f.File(
				"Tiltfile", fmt.Sprintf(`
c = %s

print(c.args)
print(c.dir)
print(c.env)
`, tc.input))

			_, err := f.ExecFile("Tiltfile")
			require.NoError(t, err)

			output := strings.Split(strings.ReplaceAll(strings.TrimSpace(f.PrintOutput()), "\r\n", "\n"), "\n")
			require.Len(t, output, 3, "Output should have been 3 lines long (args, dir, env)")

			var expectedArgs []string
			if argv, ok := tc.args.([]string); ok {
				// arguments were passed as an argv array, keep them as-is
				expectedArgs = argv
			} else if script, ok := tc.args.(string); ok {
				if runtime.GOOS == "windows" {
					expectedArgs = []string{"cmd", "/S", "/C", script}
				} else {
					expectedArgs = []string{"sh", "-c", script}
				}
			} else {
				require.Fail(t, "Bad args type %T: %v", tc.args, tc.args)
			}

			// Starlark's stringified output of a string list is JSON-compatible,
			// so we use that for args + env to get better assertion output in the
			// tests vs just string comparisons
			var actualArgs []string
			require.NoError(t, json.Unmarshal([]byte(output[0]), &actualArgs),
				"Could not read args from output: %s", output[0])
			assert.Equal(t, expectedArgs, actualArgs, "Cmd args did not match")

			expectedDir := tc.dir
			if expectedDir == "" {
				expectedDir = f.Path()
			}
			assert.Equal(t, expectedDir, output[1], "Dir did not match")

			expectedEnv := tc.env
			if expectedEnv == nil {
				// the helpers will always give us an empty list instead of nil
				// but the distinction is irrelevant in usage
				expectedEnv = make([]string, 0)
			}
			var actualEnv []string
			require.NoError(t, json.Unmarshal([]byte(output[2]), &actualEnv),
				"Could not read env from output: %s", output[2])
			assert.Equal(t, expectedEnv, actualEnv, "Env did not match")
		})
	}
}
