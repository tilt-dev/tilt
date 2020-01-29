package dockerprune

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/docker/go-units"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"

	"github.com/windmilleng/tilt/internal/container"

	"github.com/windmilleng/tilt/pkg/model"

	"github.com/windmilleng/tilt/internal/engine/buildcontrol"

	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/sliceutils"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
)

type DockerPruner struct {
	dCli docker.Client

	disabledForTesting bool
	disabledOnSetup    bool

	lastPruneBuildCount int
	lastPruneTime       time.Time
}

var _ store.Subscriber = &DockerPruner{}
var _ store.SetUpper = &DockerPruner{}

func NewDockerPruner(dCli docker.Client) *DockerPruner {
	return &DockerPruner{dCli: dCli}
}

func (dp *DockerPruner) DisabledForTesting(disabled bool) {
	dp.disabledForTesting = disabled
}

func (dp *DockerPruner) SetUp(ctx context.Context) {
	err := dp.dCli.CheckConnected()
	if err != nil {
		// If Docker is not responding at all, other parts of the system will log this.
		dp.disabledOnSetup = true
		return
	}

	if err := dp.sufficientVersionError(); err != nil {
		logger.Get(ctx).Infof(
			"[Docker Prune] Docker API version too low for Tilt to run Docker Prune:\n\t%v", err,
		)
		dp.disabledOnSetup = true
		return
	}
}

func (dp *DockerPruner) OnChange(ctx context.Context, st store.RStore) {
	if dp.disabledForTesting || dp.disabledOnSetup {
		return
	}

	state := st.RLockState()
	settings := state.DockerPruneSettings
	buildInProg := len(state.CurrentlyBuilding) > 0
	curBuildCount := state.CompletedBuildCount
	hasDockerBuild := state.HasDockerBuild()
	nextToBuild := buildcontrol.NextManifestNameToBuild(state)
	imgSelectors := model.LocalRefSelectorsForManifests(state.Manifests())
	st.RUnlockState()

	if !settings.Enabled {
		return
	}

	// If user doesn't have at least one Docker build, they probably don't care about pruning
	if !hasDockerBuild {
		return
	}

	// Don't prune while we're building or about to build something, in case of weird side-effects.
	if buildInProg || nextToBuild != "" {
		return
	}

	// Prune as soon after startup as we can (waiting until we've built SOMETHING)
	if dp.lastPruneTime.IsZero() && curBuildCount > 0 {
		dp.PruneAndRecordState(ctx, settings.MaxAge, imgSelectors, curBuildCount)
		return
	}

	// "Prune every X builds" takes precedence over "prune every Y hours"
	if settings.NumBuilds != 0 {
		buildsSince := curBuildCount - dp.lastPruneBuildCount
		if buildsSince >= settings.NumBuilds {
			dp.PruneAndRecordState(ctx, settings.MaxAge, imgSelectors, curBuildCount)
		}
		return
	}

	interval := settings.Interval
	if interval == 0 {
		interval = model.DockerPruneDefaultInterval
	}

	if time.Since(dp.lastPruneTime) >= interval {
		dp.PruneAndRecordState(ctx, settings.MaxAge, imgSelectors, curBuildCount)
	}
}

func (dp *DockerPruner) PruneAndRecordState(ctx context.Context, maxAge time.Duration, imgSelectors []container.RefSelector, curBuildCount int) {
	dp.Prune(ctx, maxAge, imgSelectors)
	dp.lastPruneTime = time.Now()
	dp.lastPruneBuildCount = curBuildCount
}

func (dp *DockerPruner) Prune(ctx context.Context, maxAge time.Duration, imgSelectors []container.RefSelector) {
	// For future: dispatch event with output/errors to be recorded
	//   in engineState.TiltSystemState on store (analogous to TiltfileState)
	err := dp.prune(ctx, maxAge, imgSelectors)
	if err != nil {
		logger.Get(ctx).Infof("[Docker Prune] error running docker prune: %v", err)
	}
}

func (dp *DockerPruner) prune(ctx context.Context, maxAge time.Duration, imgSelectors []container.RefSelector) error {
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
	imageReport, err := dp.deleteOldImages(ctx, maxAge, imgSelectors)
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

func (dp *DockerPruner) deleteOldImages(ctx context.Context, maxAge time.Duration, selectors []container.RefSelector) (types.ImagesPruneReport, error) {
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
			logger.Get(ctx).Debugf("[Docker Prune] error inspecting image '%s': %v", imgSummary.ID, err)
			continue
		}

		namedRefs, err := container.ParseNamedMulti(inspect.RepoTags)
		if err != nil {
			logger.Get(ctx).Debugf("[Docker Prune] error parsing repo tags for '%s': %v", imgSummary.ID, err)
			continue
		}

		if time.Since(inspect.Metadata.LastTagTime) >= maxAge && container.AnyMatch(namedRefs, selectors) {
			if len(inspect.RepoTags) > 1 {
				logger.Get(ctx).Debugf("[Docker Prune] cannot prune image %s (tags: %s); `docker image remove --force` "+
					"required to remove an image with multiple tags (Docker throws error: "+
					"\"image is referenced in one or more repositories\")",
					inspect.ID, strings.Join(inspect.RepoTags, ", "))
				continue
			}
			toDelete[inspect.ID] = uint64(inspect.Size)
		}
	}

	rmOpts := types.ImageRemoveOptions{PruneChildren: true}
	var responseItems []types.ImageDeleteResponseItem
	var reclaimedBytes uint64

	for imgID, bytes := range toDelete {
		items, err := dp.dCli.ImageRemove(ctx, imgID, rmOpts)
		if err != nil {
			// No good way to detect in-use images from `inspect` output, so just ignore those errors
			if !strings.Contains(err.Error(), "image is being used by running container") {
				logger.Get(ctx).Debugf("[Docker Prune] error removing image '%s': %v", imgID, err)
			}
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
	if len(report.ImagesDeleted) == 0 && !l.Level().ShouldDisplay(logger.DebugLvl) {
		return
	}

	l.Infof("[Docker Prune] removed %d images, reclaimed %s",
		len(report.ImagesDeleted), humanSize(report.SpaceReclaimed))
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
	if len(report.CachesDeleted) == 0 && !l.Level().ShouldDisplay(logger.DebugLvl) {
		return
	}

	l.Infof("[Docker Prune] removed %d caches, reclaimed %s",
		len(report.CachesDeleted), humanSize(report.SpaceReclaimed))
	if len(report.CachesDeleted) > 0 {
		l.Debugf(sliceutils.BulletedIndentedStringList(report.CachesDeleted))
	}
}

func prettyPrintContainersPruneReport(report types.ContainersPruneReport, l logger.Logger) {
	if len(report.ContainersDeleted) == 0 && !l.Level().ShouldDisplay(logger.DebugLvl) {
		return
	}

	l.Infof("[Docker Prune] removed %d containers, reclaimed %s",
		len(report.ContainersDeleted), humanSize(report.SpaceReclaimed))
	if len(report.ContainersDeleted) > 0 {
		l.Debugf(sliceutils.BulletedIndentedStringList(report.ContainersDeleted))
	}
}

func humanSize(bytes uint64) string {
	return units.HumanSize(float64(bytes))
}
