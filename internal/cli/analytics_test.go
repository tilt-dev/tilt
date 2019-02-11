package cli

import (
	"os/exec"
	"testing"

	"github.com/windmilleng/tilt/internal/testutils/tempdir"

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

func TestGitOrigin(t *testing.T) {
	tf := tempdir.NewTempDirFixture(t)
	defer tf.TearDown()

	err := exec.Command("git", "init", tf.Path()).Run()
	if err != nil {
		t.Fatalf("failed to init git repo: %+v", err)
	}
	err = exec.Command("git", "-C", tf.Path(), "remote", "add", "origin", "https://github.com/windmilleng/tilt").Run()
	if err != nil {
		t.Fatalf("failed to set origin's url: %+v", err)
	}
	origin := gitOrigin(tf.Path())

	// we can't just compare raw urls because of https://git-scm.com/docs/git-config#git-config-urlltbasegtinsteadOf
	// e.g., circleci images set `url.ssh://git@github.com.insteadof=https://github.com`
	assert.Equal(t, "//github.com/windmilleng/tilt", normalizeGitRemote(origin))
}
