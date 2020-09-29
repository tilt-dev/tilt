package dockerprune

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestDockerPrune(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
docker_prune_settings(disable=True, max_age_mins=1)
`)
	result, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.False(t, MustState(result).Enabled)
	assert.Equal(t, time.Minute, MustState(result).MaxAge)

	f.File("Tiltfile.empty", `
`)
	result, err = f.ExecFile("Tiltfile.empty")
	assert.NoError(t, err)
	assert.True(t, MustState(result).Enabled)
	assert.Equal(t, model.DockerPruneDefaultMaxAge, MustState(result).MaxAge)
}

func TestDockerPruneKeepRecent(t *testing.T) {
	f := NewFixture(t)
	f.File("Tiltfile", `
docker_prune_settings(keep_recent=5)
`)
	result, err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.True(t, MustState(result).Enabled)
	assert.Equal(t, 5, MustState(result).KeepRecent)

	f.File("Tiltfile.empty", `
`)
	result, err = f.ExecFile("Tiltfile.empty")
	assert.NoError(t, err)
	assert.Equal(t, model.DockerPruneDefaultKeepRecent, MustState(result).KeepRecent)
}

func NewFixture(tb testing.TB) *starkit.Fixture {
	return starkit.NewFixture(tb, NewExtension())
}
