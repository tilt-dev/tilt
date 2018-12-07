package engine

import (
	"context"
	"time"

	"github.com/pkg/errors"

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

func (c *ImageController) manifestsToReap(st store.RStore) []model.Manifest {
	state := st.RLockState()
	defer st.RUnlockState()
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
		if ms.Manifest.DockerRef() == nil {
			continue
		}
		manifests = append(manifests, ms.Manifest)
	}
	return manifests
}

func (c *ImageController) OnChange(ctx context.Context, st store.RStore) {
	manifestsToReap := c.manifestsToReap(st)
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
	watchFilter := build.FilterByLabelValue(build.BuildMode, build.BuildModeExisting)
	for _, manifest := range manifests {
		ref := manifest.DockerRef()
		if ref == nil {
			continue
		}
		nameFilter := build.FilterByRefName(ref)
		err := c.reaper.RemoveTiltImages(ctx, createdBefore, false, watchFilter, nameFilter)
		if err != nil {
			return errors.Wrap(err, "reapOldWatchBuilds")
		}
	}

	return nil
}
