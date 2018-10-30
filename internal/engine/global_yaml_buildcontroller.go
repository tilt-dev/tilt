package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type GlobalYAMLBuildController struct {
	disabledForTesting     bool
	lastGlobalYAMLManifest model.YAMLManifest
}

func NewGlobalYAMLBuildController() *GlobalYAMLBuildController {
	return &GlobalYAMLBuildController{}
}

func (c *GlobalYAMLBuildController) OnChange(ctx context.Context, st *store.Store) {
	if c.disabledForTesting {
		return
	}

	state := st.RLockState()
	defer st.RUnlockState()

	if !state.GlobalYAML.Equal(c.lastGlobalYAMLManifest) {
		// TOOD(dmiller) do a build
		logger.Get(ctx).Debugf("Gotta rebuild/apply the global YAML!")

		// assuming it succeeds
		c.lastGlobalYAMLManifest = state.GlobalYAML
	}
}
