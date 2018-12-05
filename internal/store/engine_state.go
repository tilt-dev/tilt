package store

import (
	"fmt"
	"net/url"
	"os"
	"sort"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
	"k8s.io/api/core/v1"
)

type EngineState struct {
	// saved so that we can render in order
	ManifestDefinitionOrder []model.ManifestName

	ManifestStates    map[model.ManifestName]*ManifestState
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

	LastTiltfileError error
}

type ManifestState struct {
	LastBuild BuildResult
	Manifest  model.Manifest
	PodSet    PodSet
	LBs       map[k8s.ServiceName]*url.URL

	// Store the times of all the pending changes,
	// so we can prioritize the oldest one first.
	PendingFileChanges    map[string]time.Time
	PendingManifestChange time.Time
	StartedFirstBuild     bool

	CurrentBuildEdits     []string
	CurrentBuildStartTime time.Time
	CurrentBuildLog       []byte `testdiff:"ignore"`
	CurrentBuildReason    model.BuildReason

	LastSuccessfulDeployTime time.Time

	LastBuildEdits      []string
	LastBuildError      error
	LastBuildStartTime  time.Time
	LastBuildFinishTime time.Time
	LastBuildReason     model.BuildReason
	LastBuildDuration   time.Duration
	LastBuildLog        []byte `testdiff:"ignore"`

	// If the pod isn't running this container then it's possible we're running stale code
	ExpectedContainerID container.ID
	// We detected stale code and are currently doing an image build
	NeedsRebuildFromCrash bool

	// If a pod had to be killed because it was crashing, we keep the old log
	// around for a little while so we can show it in the UX.
	CrashLog string
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
		PendingFileChanges: make(map[string]time.Time),
		LBs:                make(map[k8s.ServiceName]*url.URL),
		CurrentBuildLog:    []byte{},
	}
}

func (ms *ManifestState) MostRecentPod() Pod {
	return ms.PodSet.MostRecentPod()
}

func (ms *ManifestState) NextBuildReason() model.BuildReason {
	reason := model.BuildReasonNone
	if len(ms.PendingFileChanges) > 0 {
		reason = reason.With(model.BuildReasonFlagMountFiles)
	}
	if !ms.PendingManifestChange.IsZero() {
		reason = reason.With(model.BuildReasonFlagConfig)
	}
	if !ms.StartedFirstBuild {
		reason = reason.With(model.BuildReasonFlagInit)
	}
	if ms.NeedsRebuildFromCrash {
		reason = reason.With(model.BuildReasonFlagCrash)
	}
	return reason
}

// Whether a change at the given time should trigger a build.
// Used to determine if changes to mount files or config files
// should kick off a new build.
func (ms *ManifestState) IsPendingTime(t time.Time) bool {
	return !t.IsZero() && t.After(ms.LastBuildStartTime)
}

// Whether changes have been made to this Manifest's mount files
// or config since the last build.
func (ms *ManifestState) PendingBuildSince() time.Time {
	earliest := time.Now()
	isPending := false

	for _, t := range ms.PendingFileChanges {
		if t.Before(earliest) && ms.IsPendingTime(t) {
			earliest = t
			isPending = true
		}
	}

	t := ms.PendingManifestChange
	if t.Before(earliest) && ms.IsPendingTime(t) {
		earliest = t
		isPending = true
	}

	if !isPending {
		return time.Time{}
	}
	return earliest
}

type YAMLManifestState struct {
	HasBeenDeployed bool

	CurrentApplyStartTime   time.Time
	LastError               error
	LastApplyFinishTime     time.Time
	LastSuccessfulApplyTime time.Time
	LastApplyDuration       time.Duration
}

func NewYAMLManifestState() *YAMLManifestState {
	return &YAMLManifestState{}
}

type PodSet struct {
	Pods    map[k8s.PodID]*Pod
	ImageID reference.NamedTagged
}

func NewPodSet(pods ...Pod) PodSet {
	podMap := make(map[k8s.PodID]*Pod, len(pods))
	for _, pod := range pods {
		p := pod
		podMap[p.PodID] = &p
	}
	return PodSet{
		Pods: podMap,
	}
}

func (s PodSet) Len() int {
	return len(s.Pods)
}

func (s PodSet) ContainsID(id k8s.PodID) bool {
	_, ok := s.Pods[id]
	return ok
}

func (s PodSet) PodList() []Pod {
	pods := make([]Pod, 0, len(s.Pods))
	for _, pod := range s.Pods {
		pods = append(pods, *pod)
	}
	return pods
}

// Get the "most recent pod" from the PodSet.
// For most users, we believe there will be only one pod per manifest.
// So most of this time, this will return the only pod.
// And in other cases, it will return a reasonable, consistent default.
func (s PodSet) MostRecentPod() Pod {
	bestPod := Pod{}
	found := false

	for _, v := range s.Pods {
		if !found || v.isAfter(bestPod) {
			bestPod = *v
			found = true
		}
	}

	return bestPod
}

type Pod struct {
	PodID     k8s.PodID
	Namespace k8s.Namespace
	StartedAt time.Time
	Status    string
	Phase     v1.PodPhase

	// Set when we get ready to replace a pod. We may do the update in-place.
	UpdateStartTime time.Time

	// If a pod is being deleted, Kubernetes marks it as Running
	// until it actually gets removed.
	Deleting bool

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

func (p Pod) Empty() bool {
	return p.PodID == ""
}

// A stable sort order for pods.
func (p Pod) isAfter(p2 Pod) bool {
	if p.StartedAt.After(p2.StartedAt) {
		return true
	} else if p2.StartedAt.After(p.StartedAt) {
		return false
	}
	return p.PodID > p2.PodID
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
		var absWatchPaths []string
		for _, p := range ms.Manifest.LocalPaths() {
			fi, err := os.Stat(p)
			if err == nil && !fi.IsDir() {
				absWatchPaths = append(absWatchPaths, p)
			} else {
				absWatchDirs = append(absWatchDirs, p)
			}
		}
		absWatchPaths = append(absWatchPaths, s.TiltfilePath)
		relWatchDirs := ospath.TryAsCwdChildren(absWatchDirs)
		relWatchPaths := ospath.TryAsCwdChildren(absWatchPaths)

		var pendingBuildEdits []string
		for f := range ms.PendingFileChanges {
			pendingBuildEdits = append(pendingBuildEdits, f)
		}

		pendingBuildEdits = shortenFileList(absWatchDirs, pendingBuildEdits)
		lastBuildEdits := shortenFileList(absWatchDirs, ms.LastBuildEdits)
		currentBuildEdits := shortenFileList(absWatchDirs, ms.CurrentBuildEdits)

		// Sort the strings to make the outputs deterministic.
		sort.Strings(pendingBuildEdits)

		lastBuildError := ""
		if ms.LastBuildError != nil {
			lastBuildError = ms.LastBuildError.Error()
		}

		endpoints := ManifestStateEndpoints(ms)

		lastBuildLog := string(ms.LastBuildLog)

		// NOTE(nick): Right now, the UX is designed to show the output exactly one
		// pod. A better UI might summarize the pods in other ways (e.g., show the
		// "most interesting" pod that's crash looping, or show logs from all pods
		// at once).
		pod := ms.MostRecentPod()
		r := view.Resource{
			Name:                  name.String(),
			DirectoriesWatched:    relWatchDirs,
			PathsWatched:          relWatchPaths,
			LastDeployTime:        ms.LastSuccessfulDeployTime,
			LastBuildEdits:        lastBuildEdits,
			LastBuildError:        lastBuildError,
			LastBuildReason:       ms.LastBuildReason,
			LastBuildStartTime:    ms.LastBuildStartTime,
			LastBuildFinishTime:   ms.LastBuildFinishTime,
			LastBuildDuration:     ms.LastBuildDuration,
			LastBuildLog:          lastBuildLog,
			PendingBuildEdits:     pendingBuildEdits,
			PendingBuildSince:     ms.PendingBuildSince(),
			PendingBuildReason:    ms.NextBuildReason(),
			CurrentBuildEdits:     currentBuildEdits,
			CurrentBuildLog:       string(ms.CurrentBuildLog),
			CurrentBuildStartTime: ms.CurrentBuildStartTime,
			CurrentBuildReason:    ms.CurrentBuildReason,
			PodName:               pod.PodID.String(),
			PodCreationTime:       pod.StartedAt,
			PodUpdateStartTime:    pod.UpdateStartTime,
			PodStatus:             pod.Status,
			PodRestarts:           pod.ContainerRestarts - pod.OldRestarts,
			PodLog:                pod.Log(),
			CrashLog:              ms.CrashLog,
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

	if s.LastTiltfileError != nil {
		ret.TiltfileErrorMessage = fmt.Sprintf("%T %v", s.LastTiltfileError, s.LastTiltfileError)
	}

	return ret
}
