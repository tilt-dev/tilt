package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
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

func (c *ImageController) manifestsToReap(store *store.Store) []model.Manifest {
	state := store.RLockState()
	defer store.RUnlockState()
	if !state.WatchMounts || len(state.ManifestStates) == 0 {
		return nil
	}

	// Only run the reaper once per invocation of Tilt
	if c.hasRunReaper {
		return nil
	}

	c.hasRunReaper = true
	manifests := make([]model.Manifest, 0, len(state.ManifestStates))
	for _, ms := range state.ManifestStates {
		manifests = append(manifests, ms.Manifest)
	}
	return manifests
}

func (c *ImageController) OnChange(ctx context.Context, store *store.Store) {
	manifestsToReap := c.manifestsToReap(store)
	if len(manifestsToReap) > 0 {
		go func() {
			err := c.reapOldWatchBuilds(ctx, manifestsToReap, time.Now())
			if err != nil {
				logger.Get(ctx).Debugf("Error garbage collecting builds: %v", err)
			}
		}()
	}
}

func (c *ImageController) reapOldWatchBuilds(ctx context.Context, manifests []model.Manifest, createdBefore time.Time) error {
	refs := make([]reference.Named, len(manifests))
	for i, s := range manifests {
		refs[i] = s.DockerRef()
	}

	watchFilter := build.FilterByLabelValue(build.BuildMode, build.BuildModeExisting)
	for _, ref := range refs {
		nameFilter := build.FilterByRefName(ref)
		err := c.reaper.RemoveTiltImages(ctx, createdBefore, false, watchFilter, nameFilter)
		if err != nil {
			return fmt.Errorf("reapOldWatchBuilds: %v", err)
		}
	}

	return nil
}
