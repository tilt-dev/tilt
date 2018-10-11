package store

import (
	"bytes"
	"strings"
	"time"

	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
)

type EngineState struct {
	// saved so that we can render in order
	ManifestDefinitionOrder []model.ManifestName

	ManifestStates    map[model.ManifestName]*ManifestState
	ManifestsToBuild  []model.ManifestName
	CurrentlyBuilding model.ManifestName
	WatchMounts       bool

	// How many builds were queued on startup (i.e., how many manifests there were)
	InitialBuildCount int

	// How many builds have been completed (pass or fail) since starting tilt
	CompletedBuildCount int

	OpenBrowserOnNextLB bool
}

type ManifestState struct {
	LastBuild                    BuildState
	Manifest                     model.Manifest
	Pod                          Pod
	Lbs                          []k8s.LoadBalancerSpec
	HasBeenBuilt                 bool
	PendingFileChanges           map[string]bool
	CurrentlyBuildingFileChanges []string

	CurrentBuildStartTime     time.Time
	CurrentBuildLog           bytes.Buffer
	LastSuccessfulDeployEdits []string
	LastError                 error
	LastBuildFinishTime       time.Time
	LastSuccessfulDeployTime  time.Time
	LastBuildDuration         time.Duration
	LastBuildLog              bytes.Buffer
	QueueEntryTime            time.Time

	// we've observed changes to the config file and need to reload it the next time we start a build
	ConfigIsDirty bool
}

func NewState() *EngineState {
	ret := &EngineState{}
	ret.ManifestStates = make(map[model.ManifestName]*ManifestState)
	return ret
}

func NewManifestState(manifest model.Manifest) *ManifestState {
	return &ManifestState{
		LastBuild:          BuildStateClean,
		Manifest:           manifest,
		PendingFileChanges: make(map[string]bool),
		Pod:                UnknownPod,
	}
}

type Pod struct {
	Name      string
	StartedAt time.Time
	Status    string
}

// manifestState.Pod will be set to this if we don't know anything about its pod
var UnknownPod = Pod{}

func shortenFileList(baseDir string, files []string) []string {
	var ret []string
	for _, f := range files {
		ret = append(ret, strings.TrimPrefix(strings.TrimPrefix(f, baseDir), "/"))
	}

	return ret
}

// Returns the manifests in order.
func (s EngineState) Manifests() []model.Manifest {
	result := make([]model.Manifest, 0)
	for _, name := range s.ManifestDefinitionOrder {
		ms := s.ManifestStates[name]
		result = append(result, ms.Manifest)
	}
	return result
}

func StateToView(s EngineState) view.View {
	ret := view.View{}

	for _, name := range s.ManifestDefinitionOrder {
		ms := s.ManifestStates[name]
		dirWatched := ""

		// TODO handle multiple mounts
		if len(ms.Manifest.Mounts) > 0 {
			dirWatched = ospath.TryAsCwdChildren([]string{ms.Manifest.Mounts[0].LocalPath})[0]
		}

		var pendingBuildEdits []string
		for f := range ms.PendingFileChanges {
			pendingBuildEdits = append(pendingBuildEdits, f)
		}

		baseDir := ""
		if len(ms.Manifest.Mounts) > 0 {
			baseDir = ms.Manifest.Mounts[0].LocalPath
		}

		pendingBuildEdits = shortenFileList(baseDir, pendingBuildEdits)
		lastDeployEdits := shortenFileList(baseDir, ms.LastSuccessfulDeployEdits)
		currentBuildEdits := shortenFileList(baseDir, ms.CurrentlyBuildingFileChanges)

		lastBuildError := ""
		if ms.LastError != nil {
			lastBuildError = ms.LastError.Error()
		}

		r := view.Resource{
			Name:                  name.String(),
			DirectoryWatched:      dirWatched,
			LastDeployTime:        ms.LastSuccessfulDeployTime,
			LastDeployEdits:       lastDeployEdits,
			LastBuildError:        lastBuildError,
			LastBuildFinishTime:   ms.LastBuildFinishTime,
			LastBuildDuration:     ms.LastBuildDuration,
			PendingBuildEdits:     pendingBuildEdits,
			PendingBuildSince:     ms.QueueEntryTime,
			CurrentBuildEdits:     currentBuildEdits,
			CurrentBuildStartTime: ms.CurrentBuildStartTime,
			PodName:               ms.Pod.Name,
			PodCreationTime:       ms.Pod.StartedAt,
			PodStatus:             ms.Pod.Status,
			Endpoint:              "",
		}

		ret.Resources = append(ret.Resources, r)
	}

	return ret
}
