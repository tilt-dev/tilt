package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/service"
)

type upperServiceCreator struct {
	manager service.Manager
	env     k8s.Env
}

func NewUpperServiceCreator(manager service.Manager, env k8s.Env) upperServiceCreator {
	return upperServiceCreator{manager: manager, env: env}
}

func (c upperServiceCreator) CreateServices(ctx context.Context, svcs []model.Service, watch bool) error {
	upper, err := NewUpper(ctx, c.manager, c.env)
	if err != nil {
		return err
	}

	err = upper.Up(ctx, svcs, watch)
	if err != nil {
		return err
	}

	return err
}

var _ model.ServiceCreator = upperServiceCreator{}
