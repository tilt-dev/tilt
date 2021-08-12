package shlex

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
)

func TestQuote(t *testing.T) {
	f := starkit.NewFixture(t, NewPlugin())

	f.File("Tiltfile", `
s = shlex.quote("foo '$FOO'")
print(shlex.quote("foo '$FOO'"))

`)

	_, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	require.Equal(t, `'foo '"'"'$FOO'"'"''
`, f.PrintOutput())
}
