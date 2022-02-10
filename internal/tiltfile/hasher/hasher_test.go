package hasher

import (
	"crypto/sha256"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/starlarkstruct"
)

const assertTilt = `
def equals(expected, observed):
	if expected != observed:
		fail("expected: '%s'. observed: '%s'" % (expected, observed))

assert = struct(equals=equals)
`

func hexSha256(x string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(x)))
}

func TestHashesTiltfile(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	contents := `print("Hello")`
	f.File("Tiltfile", contents)

	model, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	state, err := GetState(model)
	require.NoError(t, err)

	hashes := state.GetHashes()
	assert.Equal(t, hexSha256(contents), hashes.TiltfileSHA256)
	assert.Equal(t, hexSha256(contents), hashes.AllFilesSHA256)
}

func TestHashesMultipleFiles(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	contents := `load('assert.tilt', 'assert')
message = "Hello"
assert.equals(True, not '')
`
	f.File("Tiltfile", contents)

	model, err := f.ExecFile("Tiltfile")
	require.NoError(t, err)

	state, err := GetState(model)
	require.NoError(t, err)

	hashes := state.GetHashes()
	assert.Equal(t, hexSha256(contents), hashes.TiltfileSHA256)

	hasher := sha256.New()
	hasher.Write([]byte(contents))
	hasher.Write([]byte(assertTilt))
	assert.Equal(t, fmt.Sprintf("%x", hasher.Sum(nil)), hashes.AllFilesSHA256)
}

func newFixture(t *testing.T) *starkit.Fixture {
	f := starkit.NewFixture(t, NewPlugin(), starlarkstruct.NewPlugin())
	f.File("assert.tilt", assertTilt)
	return f
}
