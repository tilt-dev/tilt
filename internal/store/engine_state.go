package store

import (
	"bytes"
	"net/url"
	"sort"
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

	PermanentError error
}

type ManifestState struct {
	LastBuild                    BuildState
	Manifest                     model.Manifest
	Pod                          Pod
	LBs                          map[k8s.ServiceName]*url.URL
	HasBeenBuilt                 bool
	PendingFileChanges           map[string]bool
	CurrentlyBuildingFileChanges []string

	CurrentBuildStartTime     time.Time
	CurrentBuildLog           *bytes.Buffer
	LastSuccessfulDeployEdits []string
	LastError                 error
	LastBuildFinishTime       time.Time
	LastSuccessfulDeployTime  time.Time
	LastBuildDuration         time.Duration
	LastBuildLog              *bytes.Buffer
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
		LBs:                make(map[k8s.ServiceName]*url.URL),
		CurrentBuildLog:    &bytes.Buffer{},
	}
}

type Pod struct {
	PodID     k8s.PodID
	StartedAt time.Time
	Status    string

	Log []byte

	// TODO(nick): Put ContainerID and ContainerName here as well.
}

func shortenFile(baseDirs []string, f string) string {
	ret := f
	for _, baseDir := range baseDirs {
		short, isChild := ospath.Child(baseDir, f)
		if isChild && len(short) < len(ret) {
			ret = short
		}
	}
	return ret
}

// for each filename in `files`, trims the longest appropriate basedir prefix off the front
func shortenFileList(baseDirs []string, files []string) []string {
	baseDirs = append([]string{}, baseDirs...)

	var ret []string
	for _, f := range files {
		ret = append(ret, shortenFile(baseDirs, f))
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

		var absWatchDirs []string
		for _, p := range ms.Manifest.Mounts {
			absWatchDirs = append(absWatchDirs, p.LocalPath)
		}
		relWatchDirs := ospath.TryAsCwdChildren(absWatchDirs)

		var pendingBuildEdits []string
		for f := range ms.PendingFileChanges {
			pendingBuildEdits = append(pendingBuildEdits, f)
		}

		pendingBuildEdits = shortenFileList(absWatchDirs, pendingBuildEdits)
		lastDeployEdits := shortenFileList(absWatchDirs, ms.LastSuccessfulDeployEdits)
		currentBuildEdits := shortenFileList(absWatchDirs, ms.CurrentlyBuildingFileChanges)

		// Sort the strings to make the outputs deterministic.
		sort.Strings(pendingBuildEdits)

		lastBuildError := ""
		if ms.LastError != nil {
			lastBuildError = ms.LastError.Error()
		}

		var endpoints []string
		for _, u := range ms.LBs {
			if u != nil {
				endpoints = append(endpoints, u.String())
			}
		}

		r := view.Resource{
			Name:                  name.String(),
			DirectoriesWatched:    relWatchDirs,
			LastDeployTime:        ms.LastSuccessfulDeployTime,
			LastDeployEdits:       lastDeployEdits,
			LastBuildError:        lastBuildError,
			LastBuildFinishTime:   ms.LastBuildFinishTime,
			LastBuildDuration:     ms.LastBuildDuration,
			PendingBuildEdits:     pendingBuildEdits,
			PendingBuildSince:     ms.QueueEntryTime,
			CurrentBuildEdits:     currentBuildEdits,
			CurrentBuildStartTime: ms.CurrentBuildStartTime,
			PodName:               ms.Pod.PodID.String(),
			PodCreationTime:       ms.Pod.StartedAt,
			PodStatus:             ms.Pod.Status,
			Endpoints:             endpoints,
		}

		ret.Resources = append(ret.Resources, r)
	}

	return ret
}
