package engine

import (
	"context"
	"time"

	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/store"
)

// How often to prune Docker images while Tilt is running
// TODO: configurable from Tiltfile for special cases
const dockerPruneInterval = time.Hour * 12

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

	state := st.RLockState()
	defer st.RUnlockState()

	// wait until state has been kinda initialized
	if !state.TiltStartTime.IsZero() && state.LastTiltfileError() == nil {
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
	// TODO:
	//   - prune images with label: builtby=tilt and older than timestamp X
	//   - (timestamp configurable in Tiltfile)
	//   - write useful output / errors to log
	//
	// For future: dispatch event with output/errors to be recorded
	//   in engineState.TiltSystemState on store (analogous to TiltfileState)
}

var _ store.Subscriber = &BuildController{}
