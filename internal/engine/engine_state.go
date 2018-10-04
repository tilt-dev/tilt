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

func (s *engineState) enqueue(n model.ManifestName) {
	s.manifestsToBuild = append(s.manifestsToBuild, n)
}

func (s *engineState) dequeueNextManifestToBuild() model.ManifestName {
	if len(s.manifestsToBuild) == 0 {
		return ""
	} else {
		ret := s.manifestsToBuild[0]

		var newManifestsToBuild []model.ManifestName
		for _, mn := range s.manifestsToBuild {
			if mn != ret {
				newManifestsToBuild = append(newManifestsToBuild, mn)
			}
		}
		s.manifestsToBuild = newManifestsToBuild
		return ret
	}
}
