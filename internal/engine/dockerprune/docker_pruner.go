package dockerprune

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"

	"github.com/windmilleng/tilt/pkg/model"

	"github.com/windmilleng/tilt/internal/engine/buildcontrol"

	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/sliceutils"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
)

type DockerPruner struct {
	dCli docker.Client

	disabledForTesting           bool
	insufficientVersionErrLogged bool

	lastPruneBuildCount int
	lastPruneTime       time.Time
}

var _ store.Subscriber = &DockerPruner{}

func NewDockerPruner(dCli docker.Client) *DockerPruner {
	return &DockerPruner{dCli: dCli}
}

func (dp *DockerPruner) DisabledForTesting(disabled bool) {
	dp.disabledForTesting = disabled
}

func (dp *DockerPruner) OnChange(ctx context.Context, st store.RStore) {
	if dp.disabledForTesting {
		return
	}

	if err := dp.sufficientVersionError(); err != nil {
		if !dp.insufficientVersionErrLogged {
			logger.Get(ctx).Infof(
				"[Docker prune] Docker API version too low for Tilt to run Docker Prune:\n\t%v", err,
			)
			dp.insufficientVersionErrLogged = true
		}
		return
	}

	state := st.RLockState()
	settings := state.DockerPruneSettings
	inProgBuild := state.CurrentlyBuilding
	curBuildCount := state.CompletedBuildCount
	hasDockerBuild := state.HasDockerBuild()
	nextToBuild := buildcontrol.NextManifestNameToBuild(state)
	st.RUnlockState()

	if !settings.Enabled {
		return
	}

	// If user doesn't have at least one Docker build, they probably don't care about pruning
	if !hasDockerBuild {
		return
	}

	// Don't prune while we're building or about to build something, in case of weird side-effects.
	if inProgBuild != "" || nextToBuild != "" {
		return
	}

	// Prune as soon after startup as we can (waiting until we've built SOMETHING)
	if dp.lastPruneTime.IsZero() && curBuildCount > 0 {
		dp.PruneAndRecordState(ctx, settings.MaxAge, curBuildCount)
		return
	}

	// "Prune every X builds" takes precedence over "prune every Y hours"
	if settings.NumBuilds != 0 {
		buildsSince := curBuildCount - dp.lastPruneBuildCount
		if buildsSince >= settings.NumBuilds {
			dp.PruneAndRecordState(ctx, settings.MaxAge, curBuildCount)
		}
		return
	}

	interval := settings.Interval
	if interval == 0 {
		interval = model.DockerPruneDefaultInterval
	}

	if time.Since(dp.lastPruneTime) >= interval {
		dp.PruneAndRecordState(ctx, settings.MaxAge, curBuildCount)
	}
}

func (dp *DockerPruner) PruneAndRecordState(ctx context.Context, maxAge time.Duration, curBuildCount int) {
	dp.Prune(ctx, maxAge)
	dp.lastPruneTime = time.Now()
	dp.lastPruneBuildCount = curBuildCount
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

	// PRUNE CONTAINERS
	containerReport, err := dp.dCli.ContainersPrune(ctx, f)
	if err != nil {
		return err
	}
	prettyPrintContainersPruneReport(containerReport, l)

	// PRUNE IMAGES
	imageReport, err := dp.deleteOldImages(ctx, maxAge)
	if err != nil {
		return err
	}
	prettyPrintImagesPruneReport(imageReport, l)

	// PRUNE BUILD CACHE
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

	return nil
}

func (dp *DockerPruner) deleteOldImages(ctx context.Context, maxAge time.Duration) (types.ImagesPruneReport, error) {
	opts := types.ImageListOptions{
		Filters: filters.NewArgs(
			filters.Arg("label", docker.BuiltByTiltLabelStr),
		),
	}
	imgs, err := dp.dCli.ImageList(ctx, opts)
	if err != nil {
		return types.ImagesPruneReport{}, err
	}

	toDelete := make(map[string]uint64) // map imageID to size in bytes
	for _, imgSummary := range imgs {
		inspect, _, err := dp.dCli.ImageInspectWithRaw(ctx, imgSummary.ID)
		if err != nil {
			logger.Get(ctx).Debugf("[Docker prune] error inspecting image '%s': %v", imgSummary.ID, err)
			continue
		}

		if time.Since(inspect.Metadata.LastTagTime) >= maxAge {
			toDelete[inspect.ID] = uint64(inspect.Size)
		}
	}

	rmOpts := types.ImageRemoveOptions{PruneChildren: true}
	var responseItems []types.ImageDeleteResponseItem
	var reclaimedBytes uint64

	for imgID, bytes := range toDelete {
		items, err := dp.dCli.ImageRemove(ctx, imgID, rmOpts)
		// No good way to detect in-use images from `inspect` output, so just silently ignore
		if err != nil && !strings.Contains(err.Error(), "image is being used by running container") {
			logger.Get(ctx).Debugf("[Docker prune] error removing image '%s': %v", imgID, err)
			continue
		}
		responseItems = append(responseItems, items...)
		reclaimedBytes += bytes
	}

	return types.ImagesPruneReport{
		ImagesDeleted:  responseItems,
		SpaceReclaimed: reclaimedBytes,
	}, nil
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
