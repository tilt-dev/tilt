package engine

import "github.com/windmilleng/tilt/internal/model"

type engineState struct {
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

func newState() *engineState {
	return &engineState{
		manifestStates:  make(map[model.ManifestName]*manifestState),
		completedBuilds: make(chan completedBuild),
	}
}

func newManifestState(manifest model.Manifest) *manifestState {
	return &manifestState{
		lastBuild:          BuildStateClean,
		manifest:           manifest,
		pendingFileChanges: make(map[string]bool),
	}
}
