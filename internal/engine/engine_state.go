package engine

import (
	"bytes"
	"strings"
	"time"

	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
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
	manifest                     model.Manifest
	pod                          Pod
	lbs                          []k8s.LoadBalancerSpec
	hasBeenBuilt                 bool
	pendingFileChanges           map[string]bool
	currentlyBuildingFileChanges []string

	currentBuildStartTime     time.Time
	currentBuildLog           bytes.Buffer
	lastSuccessfulDeployEdits []string
	lastError                 error
	lastBuildFinishTime       time.Time
	lastSuccessfulDeployTime  time.Time
	lastBuildDuration         time.Duration
	lastBuildLog              bytes.Buffer
	queueEntryTime            time.Time

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
		pod:                unknownPod,
	}
}

type Pod struct {
	Name      string
	StartedAt time.Time
	Status    string
}

// manifestState.Pod will be set to this if we don't know anything about its pod
var unknownPod = Pod{Name: "no pod yet found"}

func shortenFileList(baseDir string, files []string) []string {
	var ret []string
	for _, f := range files {
		ret = append(ret, strings.TrimPrefix(strings.TrimPrefix(f, baseDir), "/"))
	}

	return ret
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

		var pendingBuildEdits []string
		for f := range ms.pendingFileChanges {
			pendingBuildEdits = append(pendingBuildEdits, f)
		}

		baseDir := ""
		if len(ms.manifest.Mounts) > 0 {
			baseDir = ms.manifest.Mounts[0].LocalPath
		}

		pendingBuildEdits = shortenFileList(baseDir, pendingBuildEdits)
		lastDeployEdits := shortenFileList(baseDir, ms.lastSuccessfulDeployEdits)
		currentBuildEdits := shortenFileList(baseDir, ms.currentlyBuildingFileChanges)

		lastBuildError := ""
		if ms.lastError != nil {
			lastBuildError = ms.lastError.Error()
		}

		r := view.Resource{
			Name:                  name.String(),
			DirectoryWatched:      dirWatched,
			LastDeployTime:        ms.lastSuccessfulDeployTime,
			LastDeployEdits:       lastDeployEdits,
			LastBuildError:        lastBuildError,
			LastBuildFinishTime:   ms.lastBuildFinishTime,
			LastBuildDuration:     ms.lastBuildDuration,
			PendingBuildEdits:     pendingBuildEdits,
			PendingBuildSince:     ms.queueEntryTime,
			CurrentBuildEdits:     currentBuildEdits,
			CurrentBuildStartTime: ms.currentBuildStartTime,
			PodName:               ms.pod.Name,
			PodCreationTime:       ms.pod.StartedAt,
			PodStatus:             ms.pod.Status,
			Endpoint:              "",
		}

		ret.Resources = append(ret.Resources, r)
	}

	return ret
}
