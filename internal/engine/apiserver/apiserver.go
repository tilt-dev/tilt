package apiserver

import (
	"context"

	_ "github.com/tilt-dev/tilt-apiserver/pkg/server/builder"
	"github.com/tilt-dev/tilt/internal/store"
)

type Controller struct {
}

func NewController() *Controller {
	return &Controller{}
}

func (c *Controller) SetUp(ctx context.Context) {
}

func (c *Controller) TearDown(ctx context.Context) {
}

// Every time the engine state changes, sync all the manifests to the apiserver.
func (c *Controller) OnChange(ctx context.Context, st store.RStore) {
}

var _ store.Subscriber = &Controller{}
var _ store.SetUpper = &Controller{}
var _ store.TearDowner = &Controller{}
