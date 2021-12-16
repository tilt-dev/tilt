package ospath

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Assume that macOS and Windows are case-insensitive, and other operating
// systems are not. This isn't strictly accurate, but is good enough for
// testing.
var isCaseInsensitive = runtime.GOOS == "darwin" || runtime.GOOS == "windows"

func TestCanonicalizeCaseInsensitive(t *testing.T) {
	f := NewOspathFixture(t)
	defer f.TearDown()

	fileA := filepath.Join("Parent", "FileA")
	f.TouchFiles([]string{fileA})

	if isCaseInsensitive {
		result, err := Canonicalize(f.JoinPath("parent", "filea"))
		require.NoError(t, err)
		assert.Equal(t, f.JoinPath("Parent", "FileA"), result)
	} else {
		result, err := Canonicalize(f.JoinPath("parent", "filea"))
		require.NoError(t, err)
		assert.Equal(t, f.JoinPath("parent", "filea"), result)
	}
}

func TestCanonicalizeCaseInsensitiveDoesNotExist(t *testing.T) {
	f := NewOspathFixture(t)
	defer f.TearDown()

	_, err := Realpath(f.JoinPath("parent", "filea"))
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "no such file or directory")
	}

	result, err := Canonicalize(f.JoinPath("parent", "filea"))
	require.NoError(t, err)
	assert.Equal(t, f.JoinPath("parent", "filea"), result)
}
