package service

import (
	"context"

	"github.com/windmilleng/tilt/internal/model"
)

// A creator that adds all new services to the manager.
type trackingCreator struct {
	delegate model.ServiceCreator
	manager  Manager
}

func TrackServices(delegate model.ServiceCreator, manager Manager) trackingCreator {
	return trackingCreator{delegate: delegate, manager: manager}
}

func (c trackingCreator) CreateServices(ctx context.Context, svcs []model.Service, watch bool) error {
	err := c.delegate.CreateServices(ctx, svcs, watch)
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

var _ model.ServiceCreator = trackingCreator{}
