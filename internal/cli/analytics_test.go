package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeGitRemoteSuffix(t *testing.T) {
	assert.Equal(t, normalizeGitRemote("https://github.com/windmilleng/tilt.git"), normalizeGitRemote("https://github.com/windmilleng/tilt"))
}

func TestNormalizeGitRemoteScheme(t *testing.T) {
	assert.Equal(t, normalizeGitRemote("https://github.com/windmilleng/tilt.git"), normalizeGitRemote("ssh://github.com/windmilleng/tilt"))
}

func TestNormalizeGitRemoteTrailingSlash(t *testing.T) {
	assert.Equal(t, normalizeGitRemote("https://github.com/windmilleng/tilt"), normalizeGitRemote("ssh://github.com/windmilleng/tilt/"))
}

func TestNormalizedGitRemoteUsername(t *testing.T) {
	assert.Equal(t, normalizeGitRemote("https://github.com/windmilleng/tilt"), normalizeGitRemote("git@github.com:windmilleng/tilt.git"))
}
