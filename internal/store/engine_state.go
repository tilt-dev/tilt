package store

import (
	"bytes"
	"context"
	"net/url"
	"sort"
	"time"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
	"k8s.io/api/core/v1"
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

	PermanentError error
}

type ManifestState struct {
	LastBuild                    BuildResult
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

	// If the pod isn't running this container then it's possible we're running stale code
	ExpectedContainerId k8s.ContainerID
	// We detected stale code and are currently doing an image build
	CrashRebuildInProg bool
	// we've observed changes to config file(s) and need to reload the manifest next time we start a build
	ConfigIsDirty bool
}

func NewState() *EngineState {
	ret := &EngineState{}
	ret.ManifestStates = make(map[model.ManifestName]*ManifestState)
	return ret
}

func NewManifestState(manifest model.Manifest) *ManifestState {
	return &ManifestState{
		LastBuild:          BuildResult{},
		Manifest:           manifest,
		PendingFileChanges: make(map[string]bool),
		LBs:                make(map[k8s.ServiceName]*url.URL),
		CurrentBuildLog:    &bytes.Buffer{},
	}
}

type Pod struct {
	PodID     k8s.PodID
	Namespace k8s.Namespace
	StartedAt time.Time
	Status    string
	Phase     v1.PodPhase

	// The log for the previously active pod, if any
	PreRestartLog []byte
	// The log for the currently active pod, if any
	Log []byte

	// Corresponds to the deployed container.
	ContainerName  k8s.ContainerName
	ContainerID    k8s.ContainerID
	ContainerPorts []int32
	ContainerReady bool

	// We want to show the user # of restarts since pod has been running current code,
	// i.e. OldRestarts - Total Restarts
	ContainerRestarts int
	OldRestarts       int // # times the pod restarted when it was running old code
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

// Returns a set of pending file changes, without config files that don't belong
// to mounts. (Changed config files show up in ms.PendingFileChanges and don't
// necessarily belong to any mounts/watched directories -- we don't want to run
// these files through a build b/c we'll pitch an error if we find un-mounted
// files at that point.)
func (ms *ManifestState) PendingFileChangesWithoutUnmountedConfigFiles(ctx context.Context) (map[string]bool, error) {
	matcher, err := ms.Manifest.ConfigMatcher()
	if err != nil {
		return nil, errors.Wrap(err, "[PendingFileChangesWithoutUnmountedConfigFiles] getting config matcher")
	}

	files := make(map[string]bool)
	for f := range ms.PendingFileChanges {
		matches, err := matcher.Matches(f, false)
		if err != nil {
			logger.Get(ctx).Infof("Error matching %s: %v", f, err)
		}
		if matches && !build.FileBelongsToMount(f, ms.Manifest.Mounts) {
			// Filter out config files that don't belong to a mount
			continue
		}
		files[f] = true
	}
	return files, nil
}

func StateToView(s EngineState) view.View {
	ret := view.View{}

	for _, name := range s.ManifestDefinitionOrder {
		ms := s.ManifestStates[name]

		var absWatchDirs []string
		for _, p := range ms.Manifest.LocalPaths() {
			absWatchDirs = append(absWatchDirs, p)
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
			PodRestarts:           ms.Pod.ContainerRestarts - ms.Pod.OldRestarts,
			Endpoints:             endpoints,
		}

		ret.Resources = append(ret.Resources, r)
	}

	return ret
}
