package engine

import (
	"context"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
)

// Handles image garbage collection.
type ImageController struct {
	reaper       build.ImageReaper
	hasRunReaper bool
}

func NewImageController(reaper build.ImageReaper) *ImageController {
	return &ImageController{
		reaper: reaper,
	}
}

func (c *ImageController) refsToReap(st store.RStore) []reference.Named {
	state := st.RLockState()
	defer st.RUnlockState()
	if !state.WatchFiles || len(state.ManifestTargets) == 0 {
		return nil
	}

	// Only run the reaper once per invocation of Tilt
	if c.hasRunReaper {
		return nil
	}

	c.hasRunReaper = true
	refs := []reference.Named{}
	for _, manifest := range state.Manifests() {
		for _, iTarget := range manifest.ImageTargets {
			refs = append(refs, iTarget.DeploymentRef)
		}
	}
	return refs
}

func (c *ImageController) OnChange(ctx context.Context, st store.RStore) {
	refsToReap := c.refsToReap(st)
	if len(refsToReap) > 0 {
		go func() {
			err := c.reapOldWatchBuilds(ctx, refsToReap, time.Now())
			if err != nil {
				logger.Get(ctx).Debugf("Error garbage collecting builds: %v", err)
			}
		}()
	}
}

func (c *ImageController) reapOldWatchBuilds(ctx context.Context, refs []reference.Named, createdBefore time.Time) error {
	watchFilter := build.FilterByLabelValue(build.BuildMode, build.BuildModeExisting)
	for _, ref := range refs {
		nameFilter := build.FilterByRefName(ref)
		err := c.reaper.RemoveTiltImages(ctx, createdBefore, false, watchFilter, nameFilter)
		if err != nil {
			return errors.Wrap(err, "reapOldWatchBuilds")
		}
	}

	return nil
}
