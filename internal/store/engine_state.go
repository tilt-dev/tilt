package store

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/windmilleng/wmclient/pkg/analytics"
	v1 "k8s.io/api/core/v1"

	"github.com/docker/distribution/reference"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
)

type EngineState struct {
	TiltBuildInfo model.TiltBuild
	TiltStartTime time.Time

	// saved so that we can render in order
	ManifestDefinitionOrder []model.ManifestName

	// TODO(nick): This will eventually be a general Target index.
	ManifestTargets map[model.ManifestName]*ManifestTarget

	CurrentlyBuilding model.ManifestName
	WatchFiles        bool

	// How many builds were queued on startup (i.e., how many manifests there were
	// after initial Tiltfile load)
	InitialBuildsQueued int

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

	TiltfilePath             string
	ConfigFiles              []string
	TiltIgnoreContents       string
	PendingConfigFileChanges map[string]time.Time

	// InitManifests is the list of manifest names that we were told to init from the CLI.
	InitManifests []model.ManifestName

	TriggerQueue []model.ManifestName

	LogTimestamps bool
	IsProfiling   bool

	LastTiltfileBuild    model.BuildRecord
	CurrentTiltfileBuild model.BuildRecord
	TiltfileCombinedLog  model.Log

	SailEnabled bool
	SailURL     string

	FirstTiltfileBuildCompleted bool

	// from GitHub
	LatestTiltBuild model.TiltBuild

	// Analytics Info
	AnalyticsOpt           analytics.Opt // changes to this field will propagate into the TiltAnalytics subscriber + we'll record them as user choice
	AnalyticsNudgeSurfaced bool          // this flag is set the first time we show the analytics nudge to the user.

	Features map[string]bool

	TeamName string
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
	return len(e.ManifestTargets) == 0
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

	// The container IDs that we've run a LiveUpdate on, if any. Their contents have
	// diverged from the image they are built on. If these container don't appear on
	// the pod, we've lost that state and need to rebuild.
	LiveUpdatedContainerIDs map[container.ID]bool

	// We detected stale code and are currently doing an image build
	NeedsRebuildFromCrash bool

	// If a pod had to be killed because it was crashing, we keep the old log
	// around for a little while so we can show it in the UX.
	CrashLog model.Log

	// The log stream for this resource
	CombinedLog model.Log `testdiff:"ignore"`

	// If this manifest was changed, which config files led to the most recent change in manifest definition
	ConfigFilesThatCausedChange []string

	K8sWarnEvents []k8s.EventWithEntity
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
		Name:                    mn,
		BuildStatuses:           make(map[model.TargetID]*BuildStatus),
		LBs:                     make(map[k8s.ServiceName]*url.URL),
		LiveUpdatedContainerIDs: container.NewIDSet(),
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
		reason = reason.With(model.BuildReasonFlagChangedFiles)
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
// Used to determine if changes to synced files or config files
// should kick off a new build.
func (ms *ManifestState) IsPendingTime(t time.Time) bool {
	return !t.IsZero() && t.After(ms.LastBuild().StartTime)
}

// Whether changes have been made to this Manifest's synced files
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
		Name: model.UnresourcedYAMLManifestName.TargetName(),
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

	// Error messages from the pod state if it's in an error state.
	StatusMessages []string

	// Set when we get ready to replace a pod. We may do the update in-place.
	UpdateStartTime time.Time

	// If a pod is being deleted, Kubernetes marks it as Running
	// until it actually gets removed.
	Deleting bool

	HasSynclet bool

	// The log for the currently active pod, if any
	CurrentLog model.Log `testdiff:"ignore"`

	Containers []Container

	// We want to show the user # of restarts since pod has been running current code,
	// i.e. OldRestarts - Total Restarts
	OldRestarts int // # times the pod restarted when it was running old code
}

type Container struct {
	Name     container.Name
	ID       container.ID
	Ports    []int32
	Ready    bool
	ImageRef reference.Named
	Restarts int
}

func (c Container) Empty() bool {
	return c.Name == "" && c.ID == ""
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

func (p Pod) Log() model.Log {
	return p.CurrentLog
}

func (p Pod) AllContainerPorts() []int32 {
	result := make([]int32, 0)
	for _, c := range p.Containers {
		result = append(result, c.Ports...)
	}
	return result
}

func (p Pod) AllContainersReady() bool {
	for _, c := range p.Containers {
		if !c.Ready {
			return false
		}
	}
	return true
}

func (p Pod) AllContainerRestarts() int {
	result := 0
	for _, c := range p.Containers {
		result += c.Restarts
	}
	return result
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

	publishedPorts := mt.Manifest.DockerComposeTarget().PublishedPorts()
	if len(publishedPorts) > 0 {
		for _, p := range publishedPorts {
			endpoints = append(endpoints, fmt.Sprintf("http://localhost:%d/", p))
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
		IsProfiling:   s.IsProfiling,
		LogTimestamps: s.LogTimestamps,
	}

	ret.Resources = append(ret.Resources, tiltfileResourceView(s))

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

		pendingBuildEdits = ospath.FileListDisplayNames(absWatchDirs, pendingBuildEdits)

		buildHistory := append([]model.BuildRecord{}, ms.BuildHistory...)
		for i, build := range buildHistory {
			build.Edits = ospath.FileListDisplayNames(absWatchDirs, build.Edits)
			buildHistory[i] = build
		}

		currentBuild := ms.CurrentBuild
		currentBuild.Edits = ospath.FileListDisplayNames(absWatchDirs, ms.CurrentBuild.Edits)

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
			TriggerMode:        mt.Manifest.TriggerMode,
			BuildHistory:       buildHistory,
			PendingBuildEdits:  pendingBuildEdits,
			PendingBuildSince:  pendingBuildSince,
			PendingBuildReason: ms.NextBuildReason(),
			CurrentBuild:       currentBuild,
			CrashLog:           ms.CrashLog,
			Endpoints:          endpoints,
			ResourceInfo:       resourceInfoView(mt),
		}

		ret.Resources = append(ret.Resources, r)
	}

	ret.Log = s.Log

	return ret
}

func tiltfileResourceView(s EngineState) view.Resource {
	ltfb := s.LastTiltfileBuild
	if !s.CurrentTiltfileBuild.Empty() {
		ltfb.Log = s.CurrentTiltfileBuild.Log
	}
	tr := view.Resource{
		Name:         view.TiltfileResourceName,
		IsTiltfile:   true,
		CurrentBuild: s.CurrentTiltfileBuild,
		BuildHistory: []model.BuildRecord{
			ltfb,
		},
	}
	if !s.CurrentTiltfileBuild.Empty() {
		tr.PendingBuildSince = s.CurrentTiltfileBuild.StartTime
	} else {
		tr.LastDeployTime = s.LastTiltfileBuild.FinishTime
	}
	if !s.LastTiltfileBuild.Empty() {
		err := s.LastTiltfileBuild.Error
		if err != nil {
			tr.CrashLog = model.NewLog(err.Error())
		}
	}
	return tr
}

func resourceInfoView(mt *ManifestTarget) view.ResourceInfoView {
	if mt.Manifest.IsUnresourcedYAMLManifest() {
		return view.YAMLResourceInfo{
			K8sResources: mt.Manifest.K8sTarget().DisplayNames,
		}
	}

	if dcState, ok := mt.State.ResourceState.(dockercompose.State); ok {
		return view.NewDCResourceInfo(mt.Manifest.DockerComposeTarget().ConfigPaths, dcState.Status, dcState.ContainerID, dcState.Log(), dcState.StartTime)
	} else {
		pod := mt.State.MostRecentPod()
		return view.K8sResourceInfo{
			PodName:            pod.PodID.String(),
			PodCreationTime:    pod.StartedAt,
			PodUpdateStartTime: pod.UpdateStartTime,
			PodStatus:          pod.Status,
			PodRestarts:        pod.AllContainerRestarts() - pod.OldRestarts,
			PodLog:             pod.CurrentLog,
			YAML:               mt.Manifest.K8sTarget().YAML,
		}
	}
}

// DockerComposeConfigPath returns the path to the docker-compose yaml file of any
// docker-compose manifests on this EngineState.
// NOTE(maia): current assumption is only one d-c.yaml per run, so we take the
// path from the first d-c manifest we see.
func (s EngineState) DockerComposeConfigPath() []string {
	for _, mt := range s.ManifestTargets {
		if mt.Manifest.IsDC() {
			return mt.Manifest.DockerComposeTarget().ConfigPaths
		}
	}
	return []string{}
}
