package store

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"sort"
	"time"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/container"
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

	// For synchronizing BuildController so that it's only
	// doing one action at a time. In the future, we might
	// want to allow it to parallelize builds better, but that
	// would require better tools for triaging output to different streams.
	BuildControllerActionCount int

	PermanentError error

	// The user has indicated they want to exit
	UserExited bool

	// The full log stream for tilt. This might deserve gc or file storage at some point.
	Log []byte `testdiff:"ignore"`

	// GlobalYAML is a special manifest that has no images, but has dependencies
	// and a bunch of YAML that is deployed when those dependencies change.
	// TODO(dmiller) in the future we may have many of these manifests, but for now it's a special case.
	GlobalYAML      model.YAMLManifest
	GlobalYAMLState *YAMLManifestState

	TiltfilePath             string
	ConfigFiles              []string
	PendingConfigFileChanges map[string]bool

	// InitManifests is the list of manifest names that we were told to init from the CLI.
	InitManifests []model.ManifestName
}

type ManifestState struct {
	LastBuild    BuildResult
	Manifest     model.Manifest
	Pod          Pod
	LBs          map[k8s.ServiceName]*url.URL
	HasBeenBuilt bool

	// TODO(nick): Maybe we should keep timestamps for the most
	// recent change to each file?
	PendingFileChanges map[string]bool

	CurrentlyBuildingFileChanges []string

	CurrentBuildStartTime     time.Time
	CurrentBuildLog           *bytes.Buffer `testdiff:"ignore"`
	LastManifestLoadError     error
	LastSuccessfulDeployEdits []string
	LastBuildError            error
	LastBuildFinishTime       time.Time
	LastSuccessfulDeployTime  time.Time
	LastBuildDuration         time.Duration
	LastBuildLog              *bytes.Buffer `testdiff:"ignore"`
	QueueEntryTime            time.Time

	// If the pod isn't running this container then it's possible we're running stale code
	ExpectedContainerID container.ID
	// We detected stale code and are currently doing an image build
	CrashRebuildInProg bool
	// we've observed changes to config file(s) and need to reload the manifest next time we start a build
	ConfigIsDirty bool
}

func NewState() *EngineState {
	ret := &EngineState{}
	ret.ManifestStates = make(map[model.ManifestName]*ManifestState)
	ret.PendingConfigFileChanges = make(map[string]bool)
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

type YAMLManifestState struct {
	Manifest        model.YAMLManifest
	HasBeenDeployed bool

	CurrentApplyStartTime   time.Time
	LastError               error
	LastApplyFinishTime     time.Time
	LastSuccessfulApplyTime time.Time
	LastApplyDuration       time.Duration
}

func NewYAMLManifestState(manifest model.YAMLManifest) *YAMLManifestState {
	return &YAMLManifestState{
		Manifest: manifest,
	}
}

type Pod struct {
	PodID     k8s.PodID
	Namespace k8s.Namespace
	StartedAt time.Time
	Status    string
	Phase     v1.PodPhase

	// The log for the previously active pod, if any
	PreRestartLog []byte `testdiff:"ignore"`
	// The log for the currently active pod, if any
	CurrentLog []byte `testdiff:"ignore"`

	// Corresponds to the deployed container.
	ContainerName  container.Name
	ContainerID    container.ID
	ContainerPorts []int32
	ContainerReady bool

	// We want to show the user # of restarts since pod has been running current code,
	// i.e. OldRestarts - Total Restarts
	ContainerRestarts int
	OldRestarts       int // # times the pod restarted when it was running old code
}

// attempting to include the most recent crash, but no preceding crashes
// (e.g., we don't want to show the same panic 20x in a crash loop)
// if the current pod has crashed, then just print the current pod
// if the current pod is live, print the current pod plus the last pod
func (p Pod) Log() string {
	var podLog string
	// if the most recent pod is up, then we want the log from the last run (if any), since it crashed
	if p.ContainerReady {
		podLog = string(p.PreRestartLog) + string(p.CurrentLog)
	} else {
		// otherwise, the most recent pod has the crash itself, so just return itself
		podLog = string(p.CurrentLog)
	}

	return podLog
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

func ManifestStateEndpoints(ms *ManifestState) (endpoints []string) {
	defer func() {
		sort.Strings(endpoints)
	}()

	// If the user specified port-forwards in the Tiltfile, we
	// assume that's what they want to see in the UI
	portForwards := ms.Manifest.PortForwards()
	if len(portForwards) > 0 {
		for _, pf := range portForwards {
			endpoints = append(endpoints, fmt.Sprintf("http://localhost:%d/", pf.LocalPort))
		}
		return endpoints
	}

	for _, u := range ms.LBs {
		if u != nil {
			endpoints = append(endpoints, u.String())
		}
	}
	return endpoints
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
		if ms.LastBuildError != nil {
			lastBuildError = ms.LastBuildError.Error()
		}

		lastManifestLoadError := ""
		if ms.LastManifestLoadError != nil {
			lastManifestLoadError = ms.LastManifestLoadError.Error()
		}

		endpoints := ManifestStateEndpoints(ms)

		lastBuildLog := ""
		if ms.LastBuildLog != nil {
			lastBuildLog = ms.LastBuildLog.String()
		}

		r := view.Resource{
			Name:                  name.String(),
			DirectoriesWatched:    relWatchDirs,
			LastDeployTime:        ms.LastSuccessfulDeployTime,
			LastDeployEdits:       lastDeployEdits,
			LastManifestLoadError: lastManifestLoadError,
			LastBuildError:        lastBuildError,
			LastBuildFinishTime:   ms.LastBuildFinishTime,
			LastBuildDuration:     ms.LastBuildDuration,
			LastBuildLog:          lastBuildLog,
			PendingBuildEdits:     pendingBuildEdits,
			PendingBuildSince:     ms.QueueEntryTime,
			CurrentBuildEdits:     currentBuildEdits,
			CurrentBuildStartTime: ms.CurrentBuildStartTime,
			PodName:               ms.Pod.PodID.String(),
			PodCreationTime:       ms.Pod.StartedAt,
			PodStatus:             ms.Pod.Status,
			PodRestarts:           ms.Pod.ContainerRestarts - ms.Pod.OldRestarts,
			PodLog:                ms.Pod.Log(),
			Endpoints:             endpoints,
		}

		ret.Resources = append(ret.Resources, r)
	}

	if s.GlobalYAML.K8sYAML() != "" {
		var absWatches []string
		for _, p := range s.GlobalYAML.Dependencies() {
			absWatches = append(absWatches, p)
		}
		relWatches := ospath.TryAsCwdChildren(absWatches)

		var lastError string

		if s.GlobalYAMLState.LastError != nil {
			lastError = s.GlobalYAMLState.LastError.Error()
		} else {
			lastError = ""
		}

		r := view.Resource{
			Name:                  s.GlobalYAML.ManifestName().String(),
			DirectoriesWatched:    relWatches,
			CurrentBuildStartTime: s.GlobalYAMLState.CurrentApplyStartTime,
			LastBuildFinishTime:   s.GlobalYAMLState.LastApplyFinishTime,
			LastBuildDuration:     s.GlobalYAMLState.LastApplyDuration,
			LastDeployTime:        s.GlobalYAMLState.LastSuccessfulApplyTime,
			LastBuildError:        lastError,
			IsYAMLManifest:        true,
		}

		ret.Resources = append(ret.Resources, r)
	}

	ret.Log = string(s.Log)

	return ret
}
