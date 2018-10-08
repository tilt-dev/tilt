package engine

import (
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/ospath"

	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/model"
)

type engineState struct {
	// saved so that we can render in order
	manifestDefinitionOrder []model.ManifestName

	manifestStates    map[model.ManifestName]*manifestState
	manifestsToBuild  []model.ManifestName
	currentlyBuilding model.ManifestName
	completedBuilds   chan completedBuild

	// How many builds were queued on startup (i.e., how many manifests there were)
	initialBuildCount int

	// How many builds have been completed (pass or fail) since starting tilt
	completedBuildCount int

	openBrowserOnNextLB bool
}

type manifestState struct {
	lastBuild                    BuildState
	pendingFileChanges           map[string]bool
	currentlyBuildingFileChanges []string
	manifest                     model.Manifest
	lbs                          []k8s.LoadBalancerSpec
	hasBeenBuilt                 bool

	// we've observed changes to the config file and need to reload it the next time we start a build
	configIsDirty bool
}

type completedBuild struct {
	result BuildResult
	err    error
}

func newState(manifests []model.Manifest) *engineState {
	ret := &engineState{
		completedBuilds: make(chan completedBuild),
	}

	ret.manifestStates = make(map[model.ManifestName]*manifestState)

	for _, m := range manifests {
		ret.manifestDefinitionOrder = append(ret.manifestDefinitionOrder, m.Name)
		ret.manifestStates[m.Name] = newManifestState(m)
	}

	return ret
}

func newManifestState(manifest model.Manifest) *manifestState {
	return &manifestState{
		lastBuild:          BuildStateClean,
		manifest:           manifest,
		pendingFileChanges: make(map[string]bool),
	}
}

func stateToView(s engineState) view.View {
	ret := view.View{}

	for _, name := range s.manifestDefinitionOrder {
		ms := s.manifestStates[name]
		dirWatched := ""

		// TODO handle multiple mounts
		if len(ms.manifest.Mounts) > 0 {
			dirWatched = ospath.TryAsCwdChildren([]string{ms.manifest.Mounts[0].LocalPath})[0]
		}

		filesChanged := ms.currentlyBuildingFileChanges
		for f := range ms.pendingFileChanges {
			filesChanged = append(filesChanged, f)
		}

		filesChanged = ospath.TryAsCwdChildren(filesChanged)

		rs := view.ResourceStatusFresh
		if len(ms.pendingFileChanges) > 0 || len(ms.currentlyBuildingFileChanges) > 0 {
			rs = view.ResourceStatusStale
		}
		r := view.Resource{
			Name:                    name.String(),
			DirectoryWatched:        dirWatched,
			LatestFileChanges:       filesChanged,
			TimeSinceLastFileChange: 0,
			Status:                  rs,
			StatusDesc:              "",
		}

		ret.Resources = append(ret.Resources, r)
	}

	return ret
}
