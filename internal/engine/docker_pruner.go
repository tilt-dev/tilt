package engine

import (
	"context"
	"time"

	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
)

// How often to prune Docker images while Tilt is running
const dockerPruneInterval = time.Hour * 12

// Prune Docker objects older than this
// TODO: configurable from Tiltfile for special cases
const dockerUntilInterval = time.Hour * 6

type DockerPruner struct {
	dCli docker.Client

	started            bool
	disabledForTesting bool
}

func NewDockerPruner(dCli docker.Client) *DockerPruner {
	return &DockerPruner{dCli: dCli}
}

func (dp *DockerPruner) DisableForTesting() {
	dp.disabledForTesting = true
}

func (dp *DockerPruner) OnChange(ctx context.Context, st store.RStore) {
	if dp.disabledForTesting {
		return
	}

	if dp.started {
		return
	}

	// TODO: if API Version < 1.30, return error(not supported)

	state := st.RLockState()
	defer st.RUnlockState()

	// wait until state has been kinda initialized, and there's at least one Docker build
	if !state.TiltStartTime.IsZero() && state.LastTiltfileError() == nil && state.HasDockerBuild() {
		dp.started = true
		go func() {
			select {
			case <-time.After(time.Minute * 2):
				dp.prune(ctx, st) // report once pretty soon after startup...
			case <-ctx.Done():
				return
			}

			for {
				select {
				case <-time.After(dockerPruneInterval):
					// and once every <interval> thereafter
					dp.prune(ctx, st)
				case <-ctx.Done():
					return
				}
			}
		}()
	}
}

func (dp *DockerPruner) prune(ctx context.Context, st store.RStore) {
	// For future: dispatch event with output/errors to be recorded
	//   in engineState.TiltSystemState on store (analogous to TiltfileState)
	err := dp.dCli.Prune(ctx, dockerUntilInterval) // TODO: get from somewhere
	if err != nil {
		logger.Get(ctx).Infof("[Docker prune] error running docker prune: %v", err)
	}
}

var _ store.Subscriber = &BuildController{}
