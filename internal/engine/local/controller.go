package local

import (
	"context"

	"github.com/windmilleng/tilt/internal/store"
)

type Controller struct {
	runner Runner
}

func NewController() *Controller {
	return &Controller{}
}

func (c *Controller) OnChange(ctx context.Context, st store.RStore) {
	specs := c.determineServeSpecs(ctx, st)
	c.runner.SetServeSpecs(ctx, specs)
}

func (c *Controller) determineServeSpecs(ctx context.Context, st store.RStore) []ServeSpec {
	state := st.RLockState()
	defer st.RUnlockState()

	var r []ServeSpec

	for _, mt := range state.Targets() {
		if !mt.Manifest.IsLocal() {
			continue
		}
		lt := mt.Manifest.LocalTarget()
		if lt.ServeCmd.Empty() {
			continue
		}
		r = append(r, ServeSpec{
			mt.Manifest.Name,
			lt.ServeCmd,
			mt.State.LastSuccessfulDeployTime,
		})
	}

	return r
}
