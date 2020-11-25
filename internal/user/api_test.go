package user

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/tilt-dev/wmclient/pkg/dirs"

	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestWriteMode(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()

	dir := dirs.NewTiltDevDirAt(f.Path())
	up := NewFilePrefs(dir)
	prefs, err := up.Get()
	assert.NoError(t, err)
	assert.Equal(t, model.MetricsDefault, prefs.MetricsMode)

	err = UpdateMetricsMode(up, model.MetricsLocal)
	assert.NoError(t, err)

	prefs, err = up.Get()
	assert.NoError(t, err)
	assert.Equal(t, model.MetricsLocal, prefs.MetricsMode)
}
