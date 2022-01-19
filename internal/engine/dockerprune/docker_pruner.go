package dockerprune

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/docker/go-units"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/tilt-dev/tilt/internal/engine/buildcontrol"

	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
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

func (dp *DockerPruner) SetUp(ctx context.Context, _ store.RStore) error {
	err := dp.dCli.CheckConnected()
	if err != nil {
		// If Docker is not responding at all, other parts of the system will log this.
		dp.disabledOnSetup = true
		return nil
	}

	if err := dp.sufficientVersionError(); err != nil {
		logger.Get(ctx).Infof(
			"[Docker Prune] Docker API version too low for Tilt to run Docker Prune:\n\t%v", err,
		)
		dp.disabledOnSetup = true
		return nil
	}
	return nil
}

// OnChange determines if any Tilt-built Docker images should be pruned based on settings and invokes the pruning
// process if necessary.
//
// Care should be taken when modifying this method to not introduce expensive operations unless necessary, as this
// is invoked for EVERY store action change batch. Because of this, the store (un)locking is done somewhat manually,
// so care must be taken to avoid locking issues.
func (dp *DockerPruner) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) error {
	if dp.disabledForTesting || dp.disabledOnSetup || summary.IsLogOnly() {
		return nil
	}

	state := st.RLockState()
	settings := state.DockerPruneSettings
	// Exit early if possible if any of the following is true:
	// 	* Pruning is disabled entirely
	// 	* Engine is currently building something
	// 	* There are NO `docker_build`s in the Tiltfile
	// 	* Something is queued for building
	if !settings.Enabled || len(state.CurrentlyBuilding) > 0 || !state.HasDockerBuild() || buildcontrol.NextManifestNameToBuild(state) != "" {
		st.RUnlockState()
		return nil
	}

	// Prune as soon after startup as we can (waiting until we've built SOMETHING)
	curBuildCount := state.CompletedBuildCount
	shouldPrune := dp.lastPruneTime.IsZero() && curBuildCount > 0
	// "Prune every X builds" takes precedence over "prune every Y hours"
	if settings.NumBuilds != 0 {
		buildsSince := curBuildCount - dp.lastPruneBuildCount
		if buildsSince >= settings.NumBuilds {
			shouldPrune = true
		}
	} else {
		interval := settings.Interval
		if interval == 0 {
			interval = model.DockerPruneDefaultInterval
		}
		if time.Since(dp.lastPruneTime) >= interval {
			shouldPrune = true
		}
	}

	if shouldPrune {
		// N.B. Only determine the ref selectors if we're actually going to prune - OnChange is called for every batch
		// 	of store events and this is a comparatively expensive operation (lots of regex), but 99% of the time this
		// 	is called, no pruning is going to happen, so avoid burning CPU cycles unnecessarily
		imgSelectors := model.LocalRefSelectorsForManifests(state.Manifests())
		st.RUnlockState()
		dp.PruneAndRecordState(ctx, settings.MaxAge, settings.KeepRecent, imgSelectors, curBuildCount)
		return nil
	}

	st.RUnlockState()
	return nil
}

func (dp *DockerPruner) PruneAndRecordState(ctx context.Context, maxAge time.Duration, keepRecent int, imgSelectors []container.RefSelector, curBuildCount int) {
	dp.Prune(ctx, maxAge, keepRecent, imgSelectors)
	dp.lastPruneTime = time.Now()
	dp.lastPruneBuildCount = curBuildCount
}

func (dp *DockerPruner) Prune(ctx context.Context, maxAge time.Duration, keepRecent int, imgSelectors []container.RefSelector) {
	// For future: dispatch event with output/errors to be recorded
	//   in engineState.TiltSystemState on store (analogous to TiltfileState)
	err := dp.prune(ctx, maxAge, keepRecent, imgSelectors)
	if err != nil {
		logger.Get(ctx).Infof("[Docker Prune] error running docker prune: %v", err)
	}
}

func (dp *DockerPruner) prune(ctx context.Context, maxAge time.Duration, keepRecent int, imgSelectors []container.RefSelector) error {
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
	imageReport, err := dp.deleteOldImages(ctx, maxAge, keepRecent, imgSelectors)
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

func (dp *DockerPruner) inspectImages(ctx context.Context, imgs []types.ImageSummary) []types.ImageInspect {
	result := []types.ImageInspect{}
	for _, imgSummary := range imgs {
		inspect, _, err := dp.dCli.ImageInspectWithRaw(ctx, imgSummary.ID)
		if err != nil {
			logger.Get(ctx).Debugf("[Docker Prune] error inspecting image '%s': %v", imgSummary.ID, err)
			continue
		}
		result = append(result, inspect)
	}
	return result
}

// Return all image objects that exceed the max age threshold.
func (dp *DockerPruner) filterImageInspectsByMaxAge(ctx context.Context, inspects []types.ImageInspect, maxAge time.Duration, selectors []container.RefSelector) []types.ImageInspect {
	result := []types.ImageInspect{}
	for _, inspect := range inspects {
		namedRefs, err := container.ParseNamedMulti(inspect.RepoTags)
		if err != nil {
			logger.Get(ctx).Debugf("[Docker Prune] error parsing repo tags for '%s': %v", inspect.ID, err)
			continue
		}

		// LastTagTime indicates the last time the image was built, which is more
		// meaningful to us than when the image was created.
		if time.Since(inspect.Metadata.LastTagTime) >= maxAge && container.AnyMatch(namedRefs, selectors) {
			if len(inspect.RepoTags) > 1 {
				logger.Get(ctx).Debugf("[Docker Prune] cannot prune image %s (tags: %s); `docker image remove --force` "+
					"required to remove an image with multiple tags (Docker throws error: "+
					"\"image is referenced in one or more repositories\")",
					inspect.ID, strings.Join(inspect.RepoTags, ", "))
				continue
			}
			result = append(result, inspect)
		}
	}
	return result
}

// Return all image objects that aren't in the N
// most recently used for each tag.
func (dp *DockerPruner) filterOutMostRecentInspects(ctx context.Context, inspects []types.ImageInspect, keepRecent int, selectors []container.RefSelector) []types.ImageInspect {
	// First, sort the images in order from most recent to least recent.
	recentFirst := append([]types.ImageInspect{}, inspects...)
	sort.SliceStable(recentFirst, func(i, j int) bool {
		// LastTagTime indicates the last time the image was built, which is more
		// meaningful to us than when the image was created.
		return recentFirst[i].Metadata.LastTagTime.After(recentFirst[j].Metadata.LastTagTime)
	})

	// Next, aggregate the images by which selector they match.
	imgsBySelector := make(map[container.RefSelector][]types.ImageInspect)
	for _, inspect := range recentFirst {
		namedRefs, err := container.ParseNamedMulti(inspect.RepoTags)
		if err != nil {
			logger.Get(ctx).Debugf("[Docker Prune] error parsing repo tags for '%s': %v", inspect.ID, err)
			continue
		}

		for _, sel := range selectors {
			if sel.MatchesAny(namedRefs) {
				imgsBySelector[sel] = append(imgsBySelector[sel], inspect)
				break
			}
		}
	}

	// Finally, keep the N most recent for each tag.
	idsToKeep := make(map[string]bool)
	for _, list := range imgsBySelector {
		for i := 0; i < keepRecent && i < len(list); i++ {
			idsToKeep[list[i].ID] = true
		}
	}

	result := []types.ImageInspect{}
	for _, inspect := range inspects {
		if !idsToKeep[inspect.ID] {
			result = append(result, inspect)
		}
	}
	return result
}

func (dp *DockerPruner) deleteOldImages(ctx context.Context, maxAge time.Duration, keepRecent int, selectors []container.RefSelector) (types.ImagesPruneReport, error) {
	opts := types.ImageListOptions{
		Filters: filters.NewArgs(
			filters.Arg("label", docker.BuiltByTiltLabelStr),
		),
	}
	imgs, err := dp.dCli.ImageList(ctx, opts)
	if err != nil {
		return types.ImagesPruneReport{}, err
	}

	inspects := dp.inspectImages(ctx, imgs)
	inspects = dp.filterImageInspectsByMaxAge(ctx, inspects, maxAge, selectors)
	toDelete := dp.filterOutMostRecentInspects(ctx, inspects, keepRecent, selectors)

	rmOpts := types.ImageRemoveOptions{PruneChildren: true}
	var responseItems []types.ImageDeleteResponseItem
	var reclaimedBytes uint64

	for _, inspect := range toDelete {
		items, err := dp.dCli.ImageRemove(ctx, inspect.ID, rmOpts)
		if err != nil {
			// No good way to detect in-use images from `inspect` output, so just ignore those errors
			if !strings.Contains(err.Error(), "image is being used by running container") {
				logger.Get(ctx).Debugf("[Docker Prune] error removing image '%s': %v", inspect.ID, err)
			}
			continue
		}
		responseItems = append(responseItems, items...)
		reclaimedBytes += uint64(inspect.Size)
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
	if len(report.ImagesDeleted) == 0 && !l.Level().ShouldDisplay(logger.InfoLvl) {
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
	if len(report.CachesDeleted) == 0 && !l.Level().ShouldDisplay(logger.InfoLvl) {
		return
	}

	l.Infof("[Docker Prune] removed %d caches, reclaimed %s",
		len(report.CachesDeleted), humanSize(report.SpaceReclaimed))
	if len(report.CachesDeleted) > 0 {
		l.Debugf("%s", sliceutils.BulletedIndentedStringList(report.CachesDeleted))
	}
}

func prettyPrintContainersPruneReport(report types.ContainersPruneReport, l logger.Logger) {
	if len(report.ContainersDeleted) == 0 && !l.Level().ShouldDisplay(logger.InfoLvl) {
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
