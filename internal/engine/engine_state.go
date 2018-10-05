package engine

import (
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
}

type manifestState struct {
	lastBuild                    BuildState
	pendingFileChanges           map[string]bool
	currentlyBuildingFileChanges []string
	manifest                     model.Manifest

	// we've observed changes to the config file and need to reload it the next time we start a build
	configIsDirty bool
}

type completedBuild struct {
	result BuildResult
	err    error
}

func newState(manifestDefinitionOrder []model.ManifestName) *engineState {
	return &engineState{
		manifestDefinitionOrder: manifestDefinitionOrder,
		manifestStates:          make(map[model.ManifestName]*manifestState),
		completedBuilds:         make(chan completedBuild),
	}
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
