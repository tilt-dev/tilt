package store

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/windmilleng/wmclient/pkg/analytics"

	tiltanalytics "github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/hud/view"
	"github.com/windmilleng/tilt/internal/ospath"
	"github.com/windmilleng/tilt/internal/token"
	"github.com/windmilleng/tilt/pkg/model"
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

	// How many builds have been completed (pass or fail) since starting tilt
	CompletedBuildCount int

	// For synchronizing BuildController so that it's only
	// doing one action at a time. In the future, we might
	// want to allow it to parallelize builds better, but that
	// would require better tools for triaging output to different streams.
	BuildControllerActionCount int

	FatalError error
	HUDEnabled bool

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

	TiltfileState ManifestState

	// from GitHub
	LatestTiltBuild model.TiltBuild

	// Analytics Info

	AnalyticsUserOpt       analytics.Opt // changes to this field will propagate into the TiltAnalytics subscriber + we'll record them as user choice
	AnalyticsTiltfileOpt   analytics.Opt // Set by the Tiltfile. Overrides the UserOpt.
	AnalyticsNudgeSurfaced bool          // this flag is set the first time we show the analytics nudge to the user.

	Features map[string]bool

	Secrets model.SecretSet

	CloudAddress string
	Token        token.Token
	TeamName     string

	TiltCloudUsername                           string
	TokenKnownUnregistered                      bool // to distinguish whether an empty TiltCloudUsername means "we haven't checked" or "we checked and the token isn't registered"
	WaitingForTiltCloudUsernamePostRegistration bool

	DockerPruneSettings model.DockerPruneSettings
}

// Merge analytics opt-in status from different sources.
// The Tiltfile opt-in takes precedence over the user opt-in.
func (e *EngineState) AnalyticsEffectiveOpt() analytics.Opt {
	if tiltanalytics.IsAnalyticsDisabledFromEnv() {
		return analytics.OptOut
	}
	if e.AnalyticsTiltfileOpt != analytics.OptDefault {
		return e.AnalyticsTiltfileOpt
	}
	return e.AnalyticsUserOpt
}

func (e *EngineState) ManifestNamesForTargetID(id model.TargetID) []model.ManifestName {
	result := make([]model.ManifestName, 0)
	for mn, mt := range e.ManifestTargets {
		manifest := mt.Manifest
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
		if manifest.LocalTarget().ID() == id {
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

func (e *EngineState) ManifestInTriggerQueue(mn model.ManifestName) bool {
	for _, queued := range e.TriggerQueue {
		if queued == mn {
			return true
		}
	}
	return false
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
	return e.TiltfileState.LastBuild().Error
}

func (e *EngineState) HasDockerBuild() bool {
	for _, m := range e.Manifests() {
		for _, targ := range m.ImageTargets {
			if targ.IsDockerBuild() {
				return true
			}
		}
	}
	return false
}

func (e *EngineState) InitialBuildsCompleted() bool {
	if e.ManifestTargets == nil || len(e.ManifestTargets) == 0 {
		return false
	}

	for _, mt := range e.ManifestTargets {
		if !mt.Manifest.TriggerMode.AutoInitial() {
			continue
		}

		ms, _ := e.ManifestState(mt.Manifest.Name)
		if ms == nil || ms.LastBuild().Empty() {
			return false
		}
	}

	return true
}

// TODO(nick): This will eventually implement TargetStatus
type BuildStatus struct {
	// Stores the times of all the pending changes,
	// so we can prioritize the oldest one first.
	// This map is mutable.
	PendingFileChanges map[string]time.Time

	LastSuccessfulResult BuildResult
	LastResult           BuildResult
}

func newBuildStatus() *BuildStatus {
	return &BuildStatus{
		PendingFileChanges: make(map[string]time.Time),
	}
}

func (s BuildStatus) IsEmpty() bool {
	return len(s.PendingFileChanges) == 0 && s.LastSuccessfulResult == nil
}

type ManifestState struct {
	Name model.ManifestName

	BuildStatuses map[model.TargetID]*BuildStatus
	RuntimeState  RuntimeState

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
}

func NewState() *EngineState {
	ret := &EngineState{}
	ret.Log = model.Log{}
	ret.ManifestTargets = make(map[model.ManifestName]*ManifestTarget)
	ret.PendingConfigFileChanges = make(map[string]time.Time)
	ret.Secrets = model.SecretSet{}
	ret.DockerPruneSettings = model.DefaultDockerPruneSettings()
	return ret
}

func newManifestState(mn model.ManifestName) *ManifestState {
	return &ManifestState{
		Name:                    mn,
		BuildStatuses:           make(map[model.TargetID]*BuildStatus),
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

func (ms *ManifestState) DCRuntimeState() dockercompose.State {
	ret, _ := ms.RuntimeState.(dockercompose.State)
	return ret
}

func (ms *ManifestState) IsDC() bool {
	_, ok := ms.RuntimeState.(dockercompose.State)
	return ok
}

func (ms *ManifestState) K8sRuntimeState() K8sRuntimeState {
	ret, _ := ms.RuntimeState.(K8sRuntimeState)
	return ret
}

func (ms *ManifestState) GetOrCreateK8sRuntimeState() K8sRuntimeState {
	ret, ok := ms.RuntimeState.(K8sRuntimeState)
	if !ok {
		ret = NewK8sRuntimeState()
		ms.RuntimeState = ret
	}
	return ret
}

func (ms *ManifestState) IsK8s() bool {
	_, ok := ms.RuntimeState.(K8sRuntimeState)
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
	return ms.K8sRuntimeState().MostRecentPod()
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

	for _, u := range mt.State.K8sRuntimeState().LBs {
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
	ret.FatalError = s.FatalError

	return ret
}

const TiltfileManifestName = model.ManifestName("(Tiltfile)")

func tiltfileResourceView(s EngineState) view.Resource {
	tr := view.Resource{
		Name:         TiltfileManifestName,
		IsTiltfile:   true,
		CurrentBuild: s.TiltfileState.CurrentBuild,
		BuildHistory: s.TiltfileState.BuildHistory,
	}
	if !s.TiltfileState.CurrentBuild.Empty() {
		tr.PendingBuildSince = s.TiltfileState.CurrentBuild.StartTime
	} else {
		tr.LastDeployTime = s.TiltfileState.LastBuild().FinishTime
	}
	if !s.TiltfileState.LastBuild().Empty() {
		err := s.TiltfileState.LastBuild().Error
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

	switch state := mt.State.RuntimeState.(type) {
	case dockercompose.State:
		return view.NewDCResourceInfo(mt.Manifest.DockerComposeTarget().ConfigPaths,
			state.Status, state.ContainerID, state.Log(), state.StartTime)
	case K8sRuntimeState:
		pod := state.MostRecentPod()
		return view.K8sResourceInfo{
			PodName:            pod.PodID.String(),
			PodCreationTime:    pod.StartedAt,
			PodUpdateStartTime: pod.UpdateStartTime,
			PodStatus:          pod.Status,
			PodRestarts:        pod.VisibleContainerRestarts(),
			PodLog:             pod.CurrentLog,
		}
	case LocalRuntimeState:
		return view.LocalResourceInfo{}
	default:
		// This is silly but it was the old behavior.
		return view.K8sResourceInfo{}
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
