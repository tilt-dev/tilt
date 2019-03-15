package store

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"time"

	v1 "k8s.io/api/core/v1"

	"github.com/docker/distribution/reference"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
)

const emptyTiltfileMsg = "Looks like you don't have any docker builds or services defined in your Tiltfile! Check out https://docs.tilt.dev/tutorial.html to get started."

type EngineState struct {
	TiltStartTime time.Time

	// saved so that we can render in order
	ManifestDefinitionOrder []model.ManifestName

	// TODO(nick): This will eventually be a general Target index.
	ManifestTargets map[model.ManifestName]*ManifestTarget

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
	Log model.Log `testdiff:"ignore"`

	// GlobalYAML is a special manifest that has no images, but has dependencies
	// and a bunch of YAML that is deployed when those dependencies change.
	// TODO(dmiller) in the future we may have many of these manifests, but for now it's a special case.
	GlobalYAML      model.Manifest
	GlobalYAMLState *YAMLManifestState

	TiltfilePath             string
	ConfigFiles              []string
	TiltIgnoreContents       string
	PendingConfigFileChanges map[string]time.Time

	// InitManifests is the list of manifest names that we were told to init from the CLI.
	InitManifests []model.ManifestName

	TriggerMode  model.TriggerMode
	TriggerQueue []model.ManifestName

	LogTimestamps bool
	IsProfiling   bool

	LastTiltfileBuild    model.BuildRecord
	CurrentTiltfileBuild model.BuildRecord
	TiltfileCombinedLog  model.Log
}

func (e *EngineState) ManifestNamesForTargetID(id model.TargetID) []model.ManifestName {
	result := make([]model.ManifestName, 0)
	for mn, state := range e.ManifestTargets {
		manifest := state.Manifest
		for _, iTarget := range manifest.ImageTargets {
			if iTarget.ID() == id {
				result = append(result, mn)
			}
		}
		if manifest.K8sTarget().ID() == id {
			result = append(result, mn)
		}
		if manifest.DockerComposeTarget().ID() == id {
			result = append(result, mn)
		}
	}
	return result
}

func (e *EngineState) BuildStatus(id model.TargetID) BuildStatus {
	mns := e.ManifestNamesForTargetID(id)
	for _, mn := range mns {
		ms := e.ManifestTargets[mn].State
		bs := ms.BuildStatus(id)
		if !bs.IsEmpty() {
			return bs
		}
	}
	return BuildStatus{}
}

func (e *EngineState) UpsertManifestTarget(mt *ManifestTarget) {
	mn := mt.Manifest.Name
	_, ok := e.ManifestTargets[mn]
	if !ok {
		e.ManifestDefinitionOrder = append(e.ManifestDefinitionOrder, mn)
	}
	e.ManifestTargets[mn] = mt
}

func (e EngineState) Manifest(mn model.ManifestName) (model.Manifest, bool) {
	m, ok := e.ManifestTargets[mn]
	if !ok {
		return model.Manifest{}, ok
	}
	return m.Manifest, ok
}

func (e EngineState) ManifestState(mn model.ManifestName) (*ManifestState, bool) {
	m, ok := e.ManifestTargets[mn]
	if !ok {
		return nil, ok
	}
	return m.State, ok
}

// Returns Manifests in a stable order
func (e EngineState) Manifests() []model.Manifest {
	result := make([]model.Manifest, 0, len(e.ManifestTargets))
	for _, mn := range e.ManifestDefinitionOrder {
		mt, ok := e.ManifestTargets[mn]
		if !ok {
			continue
		}
		result = append(result, mt.Manifest)
	}
	return result
}

// Returns ManifestStates in a stable order
func (e EngineState) ManifestStates() []*ManifestState {
	result := make([]*ManifestState, 0, len(e.ManifestTargets))
	for _, mn := range e.ManifestDefinitionOrder {
		mt, ok := e.ManifestTargets[mn]
		if !ok {
			continue
		}
		result = append(result, mt.State)
	}
	return result
}

// Returns ManifestTargets in a stable order
func (e EngineState) Targets() []*ManifestTarget {
	result := make([]*ManifestTarget, 0, len(e.ManifestTargets))
	for _, mn := range e.ManifestDefinitionOrder {
		mt, ok := e.ManifestTargets[mn]
		if !ok {
			continue
		}
		result = append(result, mt)
	}
	return result
}

func (e EngineState) RelativeTiltfilePath() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return filepath.Rel(wd, e.TiltfilePath)
}

func (e EngineState) IsEmpty() bool {
	return len(e.ManifestTargets) == 0 && e.GlobalYAML.Name == ""
}

func (e EngineState) LastTiltfileError() error {
	return e.LastTiltfileBuild.Error
}

type ResourceState interface {
	ResourceState()
}

// TODO(nick): This will eventually implement TargetStatus
type BuildStatus struct {
	// Stores the times of all the pending changes,
	// so we can prioritize the oldest one first.
	// This map is mutable.
	PendingFileChanges map[string]time.Time

	LastSuccessfulResult BuildResult
}

func newBuildStatus() *BuildStatus {
	return &BuildStatus{
		PendingFileChanges: make(map[string]time.Time),
	}
}

func (s BuildStatus) IsEmpty() bool {
	return len(s.PendingFileChanges) == 0 && s.LastSuccessfulResult.IsEmpty()
}

type ManifestState struct {
	Name model.ManifestName

	// k8s-specific state
	PodSet   PodSet
	LBs      map[k8s.ServiceName]*url.URL
	DeployID model.DeployID // ID we have assigned to the current deploy (helps find expected k8s objects)

	BuildStatuses map[model.TargetID]*BuildStatus

	// State of the running resource -- specific to type (e.g. k8s, docker-compose, etc.)
	ResourceState ResourceState

	PendingManifestChange time.Time

	// The current build
	CurrentBuild model.BuildRecord

	LastSuccessfulDeployTime time.Time

	// The last `BuildHistoryLimit` builds. The most recent build is first in the slice.
	BuildHistory []model.BuildRecord

	// If the pod isn't running this container then it's possible we're running stale code
	ExpectedContainerID container.ID
	// We detected stale code and are currently doing an image build
	NeedsRebuildFromCrash bool

	// If a pod had to be killed because it was crashing, we keep the old log
	// around for a little while so we can show it in the UX.
	CrashLog model.Log

	// The log stream for this resource
	CombinedLog model.Log `testdiff:"ignore"`

	// If this manifest was changed, which config files led to the most recent change in manifest definition
	ConfigFilesThatCausedChange []string
}

func NewState() *EngineState {
	ret := &EngineState{}
	ret.Log = model.Log{}
	ret.ManifestTargets = make(map[model.ManifestName]*ManifestTarget)
	ret.PendingConfigFileChanges = make(map[string]time.Time)
	return ret
}

func newManifestState(mn model.ManifestName) *ManifestState {
	return &ManifestState{
		Name:          mn,
		BuildStatuses: make(map[model.TargetID]*BuildStatus),
		LBs:           make(map[k8s.ServiceName]*url.URL),
	}
}

func (ms *ManifestState) TargetID() model.TargetID {
	return model.TargetID{
		Type: model.TargetTypeManifest,
		Name: ms.Name.TargetName(),
	}
}

func (ms *ManifestState) BuildStatus(id model.TargetID) BuildStatus {
	result, ok := ms.BuildStatuses[id]
	if !ok {
		return BuildStatus{}
	}
	return *result
}

func (ms *ManifestState) MutableBuildStatus(id model.TargetID) *BuildStatus {
	result, ok := ms.BuildStatuses[id]
	if !ok {
		result = newBuildStatus()
		ms.BuildStatuses[id] = result
	}
	return result
}

func (ms *ManifestState) DCResourceState() dockercompose.State {
	ret, _ := ms.ResourceState.(dockercompose.State)
	return ret
}

func (ms *ManifestState) IsDC() bool {
	_, ok := ms.ResourceState.(dockercompose.State)
	return ok
}

func (ms *ManifestState) ActiveBuild() model.BuildRecord {
	return ms.CurrentBuild
}

func (ms *ManifestState) LastBuild() model.BuildRecord {
	if len(ms.BuildHistory) == 0 {
		return model.BuildRecord{}
	}
	return ms.BuildHistory[0]
}

func (ms *ManifestState) AddCompletedBuild(bs model.BuildRecord) {
	ms.BuildHistory = append([]model.BuildRecord{bs}, ms.BuildHistory...)
	if len(ms.BuildHistory) > model.BuildHistoryLimit {
		ms.BuildHistory = ms.BuildHistory[:model.BuildHistoryLimit]
	}
}

func (ms *ManifestState) StartedFirstBuild() bool {
	return !ms.CurrentBuild.Empty() || len(ms.BuildHistory) > 0
}

func (ms *ManifestState) MostRecentPod() Pod {
	return ms.PodSet.MostRecentPod()
}

func (ms *ManifestState) HasPendingFileChanges() bool {
	for _, status := range ms.BuildStatuses {
		if len(status.PendingFileChanges) > 0 {
			return true
		}
	}
	return false
}

func (ms *ManifestState) NextBuildReason() model.BuildReason {
	reason := model.BuildReasonNone
	if ms.HasPendingFileChanges() {
		reason = reason.With(model.BuildReasonFlagMountFiles)
	}
	if !ms.PendingManifestChange.IsZero() {
		reason = reason.With(model.BuildReasonFlagConfig)
	}
	if !ms.StartedFirstBuild() {
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
	return !t.IsZero() && t.After(ms.LastBuild().StartTime)
}

// Whether changes have been made to this Manifest's mount files
// or config since the last build.
//
// Returns:
// bool: whether changes have been made
// Time: the time of the earliest change
func (ms *ManifestState) HasPendingChanges() (bool, time.Time) {
	return ms.HasPendingChangesBefore(time.Now())
}

// Like HasPendingChanges, but relative to a particular time.
func (ms *ManifestState) HasPendingChangesBefore(highWaterMark time.Time) (bool, time.Time) {
	ok := false
	earliest := highWaterMark
	t := ms.PendingManifestChange
	if t.Before(earliest) && ms.IsPendingTime(t) {
		ok = true
		earliest = t
	}

	for _, status := range ms.BuildStatuses {
		for _, t := range status.PendingFileChanges {
			if t.Before(earliest) && ms.IsPendingTime(t) {
				ok = true
				earliest = t
			}
		}
	}
	if !ok {
		return ok, time.Time{}
	}
	return ok, earliest
}

var _ model.TargetStatus = &ManifestState{}

type YAMLManifestState struct {
	HasBeenDeployed bool

	CurrentApplyStartTime   time.Time
	LastError               error
	LastApplyFinishTime     time.Time
	LastSuccessfulApplyTime time.Time
	LastApplyStartTime      time.Time
}

func NewYAMLManifestState() *YAMLManifestState {
	return &YAMLManifestState{}
}

func (s *YAMLManifestState) TargetID() model.TargetID {
	return model.TargetID{
		Type: model.TargetTypeManifest,
		Name: model.GlobalYAMLManifestName.TargetName(),
	}
}

func (s *YAMLManifestState) ActiveBuild() model.BuildRecord {
	return model.BuildRecord{
		StartTime: s.CurrentApplyStartTime,
	}
}

func (s *YAMLManifestState) LastBuild() model.BuildRecord {
	return model.BuildRecord{
		StartTime:  s.LastApplyStartTime,
		FinishTime: s.LastApplyFinishTime,
		Error:      s.LastError,
	}
}

var _ model.TargetStatus = &YAMLManifestState{}

type PodSet struct {
	Pods     map[k8s.PodID]*Pod
	DeployID model.DeployID // Deploy that these pods correspond to
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

	HasSynclet bool

	// The log for the previously active pod, if any
	PreRestartLog model.Log `testdiff:"ignore"`
	// The log for the currently active pod, if any
	CurrentLog model.Log `testdiff:"ignore"`

	// Corresponds to the deployed container.
	ContainerName     container.Name
	ContainerID       container.ID
	ContainerPorts    []int32
	ContainerReady    bool
	ContainerImageRef reference.Named

	// We want to show the user # of restarts since pod has been running current code,
	// i.e. OldRestarts - Total Restarts
	ContainerRestarts int
	OldRestarts       int // # times the pod restarted when it was running old code

	// HACK(maia): eventually we'll want our model of the world to handle pods with
	// multiple containers (for logs, restart counts, port forwards, etc.). For now,
	// we need to ship log visibility into multiple containers. Here's the minimum
	// of info we need for that.
	ContainerInfos []ContainerInfo
}

// The minimum info we need to retrieve logs for a container.
type ContainerInfo struct {
	ID container.ID
	container.Name
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
		podLog = p.PreRestartLog.String() + p.CurrentLog.String()
	} else {
		// otherwise, the most recent pod has the crash itself, so just return itself
		podLog = p.CurrentLog.String()
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

func ManifestTargetEndpoints(mt *ManifestTarget) (endpoints []string) {
	defer func() {
		sort.Strings(endpoints)
	}()

	// If the user specified port-forwards in the Tiltfile, we
	// assume that's what they want to see in the UI
	portForwards := mt.Manifest.K8sTarget().PortForwards
	if len(portForwards) > 0 {
		for _, pf := range portForwards {
			endpoints = append(endpoints, fmt.Sprintf("http://localhost:%d/", pf.LocalPort))
		}
		return endpoints
	}

	for _, u := range mt.State.LBs {
		if u != nil {
			endpoints = append(endpoints, u.String())
		}
	}
	return endpoints
}

func StateToView(s EngineState) view.View {
	ret := view.View{
		TriggerMode:   s.TriggerMode,
		IsProfiling:   s.IsProfiling,
		LogTimestamps: s.LogTimestamps,
	}

	for _, name := range s.ManifestDefinitionOrder {
		mt, ok := s.ManifestTargets[name]
		if !ok {
			continue
		}

		ms := mt.State

		var absWatchDirs []string
		var absWatchPaths []string
		for _, p := range mt.Manifest.LocalPaths() {
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
		for _, status := range ms.BuildStatuses {
			for f := range status.PendingFileChanges {
				pendingBuildEdits = append(pendingBuildEdits, f)
			}
		}

		pendingBuildEdits = shortenFileList(absWatchDirs, pendingBuildEdits)

		buildHistory := append([]model.BuildRecord{}, ms.BuildHistory...)
		for i, build := range buildHistory {
			build.Edits = shortenFileList(absWatchDirs, build.Edits)
			buildHistory[i] = build
		}

		currentBuild := ms.CurrentBuild
		currentBuild.Edits = shortenFileList(absWatchDirs, ms.CurrentBuild.Edits)

		// Sort the strings to make the outputs deterministic.
		sort.Strings(pendingBuildEdits)

		endpoints := ManifestTargetEndpoints(mt)

		// NOTE(nick): Right now, the UX is designed to show the output exactly one
		// pod. A better UI might summarize the pods in other ways (e.g., show the
		// "most interesting" pod that's crash looping, or show logs from all pods
		// at once).
		_, pendingBuildSince := ms.HasPendingChanges()
		r := view.Resource{
			Name:               name,
			DirectoriesWatched: relWatchDirs,
			PathsWatched:       relWatchPaths,
			LastDeployTime:     ms.LastSuccessfulDeployTime,
			BuildHistory:       buildHistory,
			PendingBuildEdits:  pendingBuildEdits,
			PendingBuildSince:  pendingBuildSince,
			PendingBuildReason: ms.NextBuildReason(),
			CurrentBuild:       currentBuild,
			CrashLog:           ms.CrashLog.String(),
			Endpoints:          endpoints,
			ResourceInfo:       resourceInfoView(mt),
			ShowBuildStatus:    len(mt.Manifest.ImageTargets) > 0 || mt.Manifest.IsDC(),
			CombinedLog:        ms.CombinedLog,
		}

		ret.Resources = append(ret.Resources, r)
	}

	if s.GlobalYAML.K8sTarget().YAML != "" {
		absWatches := append([]string{}, s.ConfigFiles...)
		relWatches := ospath.TryAsCwdChildren(absWatches)

		r := view.Resource{
			Name:               s.GlobalYAML.ManifestName(),
			DirectoriesWatched: relWatches,
			CurrentBuild:       s.GlobalYAMLState.ActiveBuild(),
			BuildHistory: []model.BuildRecord{
				s.GlobalYAMLState.LastBuild(),
			},
			LastDeployTime: s.GlobalYAMLState.LastSuccessfulApplyTime,
			ResourceInfo: view.YAMLResourceInfo{
				K8sResources: s.GlobalYAML.K8sTarget().ResourceNames,
			},
		}

		ret.Resources = append(ret.Resources, r)
	}

	ltfb := s.LastTiltfileBuild
	if !s.CurrentTiltfileBuild.Empty() {
		ltfb.Log = s.CurrentTiltfileBuild.Log
	}
	tr := view.Resource{
		Name:         "(Tiltfile)",
		IsTiltfile:   true,
		CurrentBuild: s.CurrentTiltfileBuild,
		BuildHistory: []model.BuildRecord{
			ltfb,
		},
		CombinedLog: s.TiltfileCombinedLog,
	}
	if !s.CurrentTiltfileBuild.Empty() {
		tr.PendingBuildSince = s.CurrentTiltfileBuild.StartTime
	} else {
		tr.LastDeployTime = s.LastTiltfileBuild.FinishTime
	}
	if !s.LastTiltfileBuild.Empty() {
		err := s.LastTiltfileBuild.Error
		if err == nil && s.IsEmpty() {
			tr.CrashLog = emptyTiltfileMsg
			ret.TiltfileErrorMessage = emptyTiltfileMsg
		} else if err != nil {
			tr.CrashLog = err.Error()
			ret.TiltfileErrorMessage = err.Error()
		}
	}
	ret.Resources = append(ret.Resources, tr)

	ret.Log = s.Log.String()

	return ret
}

func resourceInfoView(mt *ManifestTarget) view.ResourceInfoView {
	if dcState, ok := mt.State.ResourceState.(dockercompose.State); ok {
		return view.NewDCResourceInfo(mt.Manifest.DockerComposeTarget().ConfigPath, dcState.Status, dcState.ContainerID, dcState.Log(), dcState.StartTime)
	} else {
		pod := mt.State.MostRecentPod()
		return view.K8SResourceInfo{
			PodName:            pod.PodID.String(),
			PodCreationTime:    pod.StartedAt,
			PodUpdateStartTime: pod.UpdateStartTime,
			PodStatus:          pod.Status,
			PodRestarts:        pod.ContainerRestarts - pod.OldRestarts,
			PodLog:             pod.Log(),
		}
	}
}

// DockerComposeConfigPath returns the path to the docker-compose yaml file of any
// docker-compose manifests on this EngineState.
// NOTE(maia): current assumption is only one d-c.yaml per run, so we take the
// path from the first d-c manifest we see.
func (s EngineState) DockerComposeConfigPath() string {
	for _, mt := range s.ManifestTargets {
		if mt.Manifest.IsDC() {
			return mt.Manifest.DockerComposeTarget().ConfigPath
		}
	}
	return ""
}
