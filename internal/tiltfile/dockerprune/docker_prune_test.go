package dockerprune

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestDockerPrune(t *testing.T) {
	f, ext := NewFixture(t)

	f.File("Tiltfile", `
docker_prune_settings(disable=True, max_age_mins=1)
`)
	err := f.ExecFile("Tiltfile")
	assert.NoError(t, err)
	assert.False(t, ext.Settings().Enabled)
	assert.Equal(t, time.Minute, ext.Settings().MaxAge)

	f.File("Tiltfile.empty", `
`)
	err = f.ExecFile("Tiltfile.empty")
	assert.NoError(t, err)
	assert.True(t, ext.Settings().Enabled)
	assert.Equal(t, model.DockerPruneDefaultMaxAge, ext.Settings().MaxAge)
}

func NewFixture(tb testing.TB) (*starkit.Fixture, *Extension) {
	ext := NewExtension()
	return starkit.NewFixture(tb, ext), ext
}
