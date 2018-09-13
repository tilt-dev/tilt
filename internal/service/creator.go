package service

import (
	"context"

	"github.com/windmilleng/tilt/internal/model"
)

// A creator that adds all new services to the manager.
type trackingCreator struct {
	delegate model.ManifestCreator
	manager  Manager
}

func TrackServices(delegate model.ManifestCreator, manager Manager) trackingCreator {
	return trackingCreator{delegate: delegate, manager: manager}
}

func (c trackingCreator) CreateManifests(ctx context.Context, svcs []model.Manifest, watch bool) error {
	err := c.delegate.CreateManifests(ctx, svcs, watch)
	if err != nil {
		return err
	}

	for _, s := range svcs {
		err := c.manager.Add(s)
		if err != nil {
			return err
		}
	}

	return nil
}

var _ model.ManifestCreator = trackingCreator{}
