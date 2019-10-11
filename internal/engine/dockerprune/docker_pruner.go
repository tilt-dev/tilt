package dockerprune

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"

	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/sliceutils"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
)

type DockerPruner struct {
	dCli docker.Client

	started            bool
	disabledForTesting bool
}

var _ store.Subscriber = &DockerPruner{}

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

	settings := state.DockerPruneSettings
	if settings.Disabled {
		return
	}

	if err := dp.sufficientVersionError(); err != nil {
		return
	}

	// wait until state has been kinda initialized, and there's at least one Docker build
	if !state.TiltStartTime.IsZero() && state.LastTiltfileError() == nil && state.HasDockerBuild() {
		dp.started = true
		go func() {
			select {
			case <-time.After(time.Minute * 2):
				dp.Prune(ctx, settings.MaxAge) // report once pretty soon after startup...
			case <-ctx.Done():
				return
			}

			for {
				select {
				case <-time.After(settings.Interval):
					// and once every <interval> thereafter
					dp.Prune(ctx, settings.MaxAge)
				case <-ctx.Done():
					return
				}
			}
		}()
	}
}

func (dp *DockerPruner) Prune(ctx context.Context, maxAge time.Duration) {
	// For future: dispatch event with output/errors to be recorded
	//   in engineState.TiltSystemState on store (analogous to TiltfileState)
	err := dp.prune(ctx, maxAge)
	if err != nil {
		logger.Get(ctx).Infof("[Docker Prune] error running docker prune: %v", err)
	}
}

func (dp *DockerPruner) prune(ctx context.Context, maxAge time.Duration) error {
	l := logger.Get(ctx)
	if err := dp.sufficientVersionError(); err != nil {
		l.Debugf("[Docker Prune] skipping Docker prune, Docker API version too low:\t%v", err)
		return nil
	}

	f := filters.NewArgs(
		filters.Arg("label", docker.BuiltByTiltLabelStr),
		filters.Arg("until", maxAge.String()),
	)

	fWithDangling := f.Clone()
	fWithDangling.Add("dangling", "0") // prune all images, not just dangling ones
	imgReport, err := dp.dCli.ImagesPrune(ctx, fWithDangling)
	if err != nil {
		return err
	}
	prettyPrintImagesPruneReport(imgReport, l)

	opts := types.BuildCachePruneOptions{Filters: f}
	cacheReport, err := dp.dCli.BuildCachePrune(ctx, opts)
	if err != nil {
		if !strings.Contains(err.Error(), `"build prune" requires API version`) {
			return err
		}
		l.Debugf("[Docker Prune] skipping build cache prune, Docker API version too low:\t%s", err)
	} else {
		prettyPrintCachePruneReport(cacheReport, l)
	}

	containerReport, err := dp.dCli.ContainersPrune(ctx, f)
	if err != nil {
		return err
	}
	prettyPrintContainersPruneReport(containerReport, l)

	return nil
}

func (dp *DockerPruner) sufficientVersionError() error {
	return dp.dCli.NewVersionError("1.30", "image | container prune with filter: label")
}

func prettyPrintImagesPruneReport(report types.ImagesPruneReport, l logger.Logger) {
	// TODO: human-readable space reclaimed
	if len(report.ImagesDeleted) == 0 {
		return
	}

	l.Infof("[Docker Prune] removed %d images, reclaimed %d bytes", len(report.ImagesDeleted), report.SpaceReclaimed)
	if len(report.ImagesDeleted) > 0 {
		for _, img := range report.ImagesDeleted {
			l.Debugf("\t- %s", prettyStringImgDeleteItem(img))
		}
	}
}

func prettyStringImgDeleteItem(img types.ImageDeleteResponseItem) string {
	if img.Deleted != "" {
		return fmt.Sprintf("deleted: %s", img.Deleted)
	}
	if img.Untagged != "" {
		return fmt.Sprintf("untagged: %s", img.Untagged)
	}
	return ""
}

func prettyPrintCachePruneReport(report *types.BuildCachePruneReport, l logger.Logger) {
	// TODO: human-readable space reclaimed
	if len(report.CachesDeleted) == 0 {
		return
	}

	l.Infof("[Docker Prune] removed %d caches, reclaimed %d bytes", len(report.CachesDeleted), report.SpaceReclaimed)
	if len(report.CachesDeleted) > 0 {
		l.Debugf(sliceutils.BulletedIndentedStringList(report.CachesDeleted))
	}
}

func prettyPrintContainersPruneReport(report types.ContainersPruneReport, l logger.Logger) {
	// TODO: human-readable space reclaimed
	if len(report.ContainersDeleted) == 0 {
		return
	}

	l.Infof("[Docker Prune] removed %d containers, reclaimed %d bytes", len(report.ContainersDeleted), report.SpaceReclaimed)
	if len(report.ContainersDeleted) > 0 {
		l.Debugf(sliceutils.BulletedIndentedStringList(report.ContainersDeleted))
	}
}
