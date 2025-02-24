package store

import (
	"fmt"
	"iter"
	"sort"
	"time"

	"github.com/tilt-dev/wmclient/pkg/analytics"

	tiltanalytics "github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/internal/timecmp"
	"github.com/tilt-dev/tilt/internal/token"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

type EngineState struct {
	TiltBuildInfo model.TiltBuild
	TiltStartTime time.Time

	// saved so that we can render in order
	ManifestDefinitionOrder []model.ManifestName

	// TODO(nick): This will eventually be a general Target index.
	ManifestTargets map[model.ManifestName]*ManifestTarget

	// Keep a set of the current builds, so we can quickly count how many
	// builds there are without looking at all builds in the list.
	CurrentBuildSet map[model.ManifestName]bool

	TerminalMode TerminalMode

	// For synchronizing BuildController -- wait until engine records all builds started
	// so far before starting another build
	BuildControllerStartCount int

	// How many builds have been completed (pass or fail) since starting tilt
	CompletedBuildCount int

	UpdateSettings model.UpdateSettings

	FatalError error

	// The user has indicated they want to exit
	UserExited bool

	// We recovered from a panic(). We need to clean up the RTY and print the error.
	PanicExited error

	// Normal process termination. Either Tilt completed all its work,
	// or it determined that it was unable to complete the work it was assigned.
	//
	// Note that ExitSignal/ExitError is never triggered in normal
	// 'tilt up`/dev mode. It's more for CI modes and tilt up --watch=false modes.
	//
	// We don't provide the ability to customize exit codes. Either the
	// process exited successfully, or with an error. In the future, we might
	// add the ability to put an exit code in the error.
	ExitSignal bool
	ExitError  error

	// All logs in Tilt, stored in a structured format.
	LogStore *logstore.LogStore `testdiff:"ignore"`

	TriggerQueue []model.ManifestName

	TiltfileDefinitionOrder []model.ManifestName
	TiltfileStates          map[model.ManifestName]*ManifestState

	SuggestedTiltVersion string
	VersionSettings      model.VersionSettings

	// Analytics Info
	AnalyticsEnvOpt        analytics.Opt
	AnalyticsUserOpt       analytics.Opt // changes to this field will propagate into the TiltAnalytics subscriber + we'll record them as user choice
	AnalyticsTiltfileOpt   analytics.Opt // Set by the Tiltfile. Overrides the UserOpt.
	AnalyticsNudgeSurfaced bool          // this flag is set the first time we show the analytics nudge to the user.

	Features map[string]bool

	Secrets model.SecretSet

	CloudAddress string
	Token        token.Token
	TeamID       string

	DockerPruneSettings model.DockerPruneSettings

	TelemetrySettings model.TelemetrySettings

	UserConfigState model.UserConfigState

	// The initialization sequence is unfortunate. Currently we have:
	// 1) Dispatch an InitAction
	// 1) InitAction sets DesiredTiltfilePath
	// 2) ConfigsController reads DesiredTiltfilePath, writes a new Tiltfile object to the APIServer
	// 4) ConfigsController dispatches a TiltfileCreateAction, to copy the apiserver data into the EngineState
	DesiredTiltfilePath string

	// KubernetesResources by name.
	// Updated to match KubernetesApply + KubernetesDiscovery
	KubernetesResources map[string]*k8sconv.KubernetesResource `json:"-"`

	// API-server-based data models. Stored in EngineState
	// to assist in migration.
	Cmds                  map[string]*Cmd                           `json:"-"`
	Tiltfiles             map[string]*v1alpha1.Tiltfile             `json:"-"`
	FileWatches           map[string]*v1alpha1.FileWatch            `json:"-"`
	KubernetesApplys      map[string]*v1alpha1.KubernetesApply      `json:"-"`
	KubernetesDiscoverys  map[string]*v1alpha1.KubernetesDiscovery  `json:"-"`
	UIResources           map[string]*v1alpha1.UIResource           `json:"-"`
	ConfigMaps            map[string]*v1alpha1.ConfigMap            `json:"-"`
	LiveUpdates           map[string]*v1alpha1.LiveUpdate           `json:"-"`
	Clusters              map[string]*v1alpha1.Cluster              `json:"-"`
	UIButtons             map[string]*v1alpha1.UIButton             `json:"-"`
	DockerComposeServices map[string]*v1alpha1.DockerComposeService `json:"-"`
	ImageMaps             map[string]*v1alpha1.ImageMap             `json:"-"`
	DockerImages          map[string]*v1alpha1.DockerImage          `json:"-"`
	CmdImages             map[string]*v1alpha1.CmdImage             `json:"-"`
}

func (e *EngineState) MainTiltfilePath() string {
	tf, ok := e.Tiltfiles[model.MainTiltfileManifestName.String()]
	if !ok {
		return ""
	}
	return tf.Spec.Path
}

// Merge analytics opt-in status from different sources.
// The Tiltfile opt-in takes precedence over the user opt-in.
func (e *EngineState) AnalyticsEffectiveOpt() analytics.Opt {
	if e.AnalyticsEnvOpt != analytics.OptDefault {
		return e.AnalyticsEnvOpt
	}
	if e.AnalyticsTiltfileOpt != analytics.OptDefault {
		return e.AnalyticsTiltfileOpt
	}
	return e.AnalyticsUserOpt
}

func (e *EngineState) ManifestNamesForTargetID(id model.TargetID) []model.ManifestName {
	if id.Type == model.TargetTypeConfigs {
		return []model.ManifestName{model.ManifestName(id.Name)}
	}

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

func (e *EngineState) IsBuilding(name model.ManifestName) bool {
	ms, ok := e.ManifestState(name)
	if !ok {
		return false
	}
	return ms.IsBuilding()
}

// Find the first build status. Only suitable for testing.
func (e *EngineState) BuildStatus(id model.TargetID) *BuildStatus {
	mns := e.ManifestNamesForTargetID(id)
	for _, mn := range mns {
		ms := e.ManifestTargets[mn].State
		bs := ms.BuildStatus(id)
		if !bs.IsEmpty() {
			return bs
		}
	}
	return &BuildStatus{}
}

func (e *EngineState) AvailableBuildSlots() int {
	currentBuildCount := len(e.CurrentBuildSet)
	if currentBuildCount >= e.UpdateSettings.MaxParallelUpdates() {
		// this could happen if user decreases max build slots while
		// multiple builds are in progress, no big deal
		return 0
	}
	return e.UpdateSettings.MaxParallelUpdates() - currentBuildCount
}

func (e *EngineState) UpsertManifestTarget(mt *ManifestTarget) {
	mn := mt.Manifest.Name
	_, ok := e.ManifestTargets[mn]
	if !ok {
		e.ManifestDefinitionOrder = append(e.ManifestDefinitionOrder, mn)
	}
	e.ManifestTargets[mn] = mt
}

func (e *EngineState) RemoveManifestTarget(mn model.ManifestName) {
	delete(e.ManifestTargets, mn)
	newOrder := []model.ManifestName{}
	for _, n := range e.ManifestDefinitionOrder {
		if n == mn {
			continue
		}
		newOrder = append(newOrder, n)
	}
	e.ManifestDefinitionOrder = newOrder
}

func (e EngineState) Manifest(mn model.ManifestName) (model.Manifest, bool) {
	m, ok := e.ManifestTargets[mn]
	if !ok {
		return model.Manifest{}, ok
	}
	return m.Manifest, ok
}

func (e EngineState) ManifestState(mn model.ManifestName) (*ManifestState, bool) {
	st, ok := e.TiltfileStates[mn]
	if ok {
		return st, ok
	}

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

// Returns TiltfileStates in a stable order.
func (e EngineState) GetTiltfileStates() []*ManifestState {
	result := make([]*ManifestState, 0, len(e.TiltfileStates))
	for _, mn := range e.TiltfileDefinitionOrder {
		mt, ok := e.TiltfileStates[mn]
		if !ok {
			continue
		}
		result = append(result, mt)
	}
	return result
}

func (e EngineState) TargetsBesides(mn model.ManifestName) []*ManifestTarget {
	targets := e.Targets()
	result := make([]*ManifestTarget, 0, len(targets))
	for _, mt := range targets {
		if mt.Manifest.Name == mn {
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

func (e *EngineState) AppendToTriggerQueue(mn model.ManifestName, reason model.BuildReason) {
	ms, ok := e.ManifestState(mn)
	if !ok {
		return
	}

	if reason == 0 {
		reason = model.BuildReasonFlagTriggerUnknown
	}

	ms.TriggerReason = ms.TriggerReason.With(reason)

	for _, queued := range e.TriggerQueue {
		if mn == queued {
			return
		}
	}
	e.TriggerQueue = append(e.TriggerQueue, mn)
}

func (e *EngineState) RemoveFromTriggerQueue(mn model.ManifestName) {
	mState, ok := e.ManifestState(mn)
	if ok {
		mState.TriggerReason = model.BuildReasonNone
	}

	for i, triggerName := range e.TriggerQueue {
		if triggerName == mn {
			e.TriggerQueue = append(e.TriggerQueue[:i], e.TriggerQueue[i+1:]...)
			break
		}
	}
}

func (e EngineState) IsEmpty() bool {
	return len(e.ManifestTargets) == 0
}

func (e EngineState) LastMainTiltfileError() error {
	st, ok := e.TiltfileStates[model.MainTiltfileManifestName]
	if !ok {
		return nil
	}

	return st.LastBuild().Error
}

func (e *EngineState) MainTiltfileState() *ManifestState {
	return e.TiltfileStates[model.MainTiltfileManifestName]
}

func (e *EngineState) HasBuild() bool {
	for _, m := range e.Manifests() {
		for _, targ := range m.ImageTargets {
			if targ.IsDockerBuild() || targ.IsCustomBuild() {
				return true
			}
		}
	}
	return false
}

func (e *EngineState) InitialBuildsCompleted() bool {
	if len(e.ManifestTargets) == 0 {
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

type BuildStatus struct {
	// We keep track of a change with two fields:
	//
	// 1) When we saw the change.
	// 2) When we consumed the change.
	//
	// A change is considered pending if the last seen timestamp is later than the
	// last consumed timestamp.
	//
	// Because these two fields are one-way ratchets (they only move forward in
	// time), they avoid race conditions (e.g., if a we received Seen and Consumed
	// events out of order).
	ConsumedChanges time.Time

	FileChanges map[string]time.Time

	LastResult BuildResult

	// Stores the times that dependencies were marked dirty, so we can prioritize
	// the oldest one first.
	//
	// Long-term, we want to process all dependencies as a build graph rather than
	// a list of manifests. Specifically, we'll build one Target at a time.  Once
	// the build completes, we'll look at all the targets that depend on it, and
	// mark DependencyChanges to indicate that they need a rebuild.
	//
	// Short-term, we only use this for cases where two manifests share a common
	// image. This only handles cross-manifest dependencies.
	//
	// This approach allows us to start working on the bookkeeping and
	// dependency-tracking in the short-term, without having to switch over to a
	// full dependency graph in one swoop.
	DependencyChanges map[model.TargetID]time.Time
}

func newBuildStatus() *BuildStatus {
	return &BuildStatus{
		FileChanges:       make(map[string]time.Time),
		DependencyChanges: make(map[model.TargetID]time.Time),
	}
}

func (s *BuildStatus) ConsumeChangesBefore(startTime time.Time) {
	if s.ConsumedChanges.IsZero() || s.ConsumedChanges.Before(startTime) {
		s.ConsumedChanges = startTime
	}

	// Garbage collect changes consumed
	// more than a minute ago.
	for path, modTime := range s.FileChanges {
		if modTime.Add(time.Minute).Before(s.ConsumedChanges) {
			delete(s.FileChanges, path)
		}
	}

	for targetID, modTime := range s.DependencyChanges {
		if modTime.Add(time.Minute).Before(s.ConsumedChanges) {
			delete(s.DependencyChanges, targetID)
		}
	}
}

func (s *BuildStatus) IsEmpty() bool {
	return len(s.FileChanges) == 0 &&
		len(s.DependencyChanges) == 0 &&
		s.LastResult == nil
}

// Count PendingFileChanges
func (s *BuildStatus) CountPendingFileChanges() int {
	count := 0
	for _ = range s.PendingFileChanges() {
		count++
	}
	return count
}

func (s *BuildStatus) PendingFileChanges() iter.Seq2[string, time.Time] {
	return func(yield func(string, time.Time) bool) {
		neverConsumed := s.ConsumedChanges.IsZero()
		for p, modTime := range s.FileChanges {
			if neverConsumed || !timecmp.BeforeOrEqual(modTime, s.ConsumedChanges) {
				if !yield(p, modTime) {
					return
				}
			}
		}
	}
}

func (s *BuildStatus) PendingFileChangesList() []string {
	var paths []string
	for p, _ := range s.PendingFileChanges() {
		paths = append(paths, p)
	}
	return paths
}

func (s *BuildStatus) PendingFileChangesSorted() []string {
	result := s.PendingFileChangesList()
	sort.Strings(result)
	return result
}

func (s *BuildStatus) PendingDependencyChanges() iter.Seq2[model.TargetID, time.Time] {
	return func(yield func(model.TargetID, time.Time) bool) {
		neverConsumed := s.ConsumedChanges.IsZero()
		for p, modTime := range s.DependencyChanges {
			if neverConsumed || !timecmp.BeforeOrEqual(modTime, s.ConsumedChanges) {
				if !yield(p, modTime) {
					return
				}
			}
		}
	}
}

func (s *BuildStatus) HasPendingFileChanges() bool {
	for _ = range s.PendingFileChanges() {
		return true
	}
	return false
}

func (s *BuildStatus) HasPendingDependencyChanges() bool {
	for _ = range s.PendingDependencyChanges() {
		return true
	}
	return false
}

type ManifestState struct {
	Name model.ManifestName

	BuildStatuses map[model.TargetID]*BuildStatus
	RuntimeState  RuntimeState

	PendingManifestChange time.Time

	// Any current builds for this manifest.
	//
	// There can be multiple simultaneous image builds + deploys + live updates
	// associated with a manifest.
	//
	// In an ideal world, we'd read these builds from the API models
	// rather than do separate bookkeeping for them.
	CurrentBuilds map[string]model.BuildRecord

	LastSuccessfulDeployTime time.Time

	// The last `BuildHistoryLimit` builds. The most recent build is first in the slice.
	BuildHistory []model.BuildRecord

	// If this manifest was changed, which config files led to the most recent change in manifest definition
	ConfigFilesThatCausedChange []string

	// If the build was manually triggered, record why.
	TriggerReason model.BuildReason

	DisableState v1alpha1.DisableState
}

func NewState() *EngineState {
	ret := &EngineState{}
	ret.LogStore = logstore.NewLogStore()
	ret.ManifestTargets = make(map[model.ManifestName]*ManifestTarget)
	ret.Secrets = model.SecretSet{}
	ret.DockerPruneSettings = model.DefaultDockerPruneSettings()
	ret.VersionSettings = model.VersionSettings{
		CheckUpdates: true,
	}
	ret.UpdateSettings = model.DefaultUpdateSettings()
	ret.CurrentBuildSet = make(map[model.ManifestName]bool)

	// For most Tiltfiles, this is created by the TiltfileUpsertAction.  But
	// lots of tests assume tha main tiltfile state exists on initialization.
	ret.TiltfileDefinitionOrder = []model.ManifestName{model.MainTiltfileManifestName}
	ret.TiltfileStates = map[model.ManifestName]*ManifestState{
		model.MainTiltfileManifestName: &ManifestState{
			Name:          model.MainTiltfileManifestName,
			BuildStatuses: make(map[model.TargetID]*BuildStatus),
			DisableState:  v1alpha1.DisableStateEnabled,
			CurrentBuilds: make(map[string]model.BuildRecord),
		},
	}

	if ok, _ := tiltanalytics.IsAnalyticsDisabledFromEnv(); ok {
		ret.AnalyticsEnvOpt = analytics.OptOut
	}

	ret.Cmds = make(map[string]*Cmd)
	ret.Tiltfiles = make(map[string]*v1alpha1.Tiltfile)
	ret.FileWatches = make(map[string]*v1alpha1.FileWatch)
	ret.KubernetesApplys = make(map[string]*v1alpha1.KubernetesApply)
	ret.DockerComposeServices = make(map[string]*v1alpha1.DockerComposeService)
	ret.KubernetesDiscoverys = make(map[string]*v1alpha1.KubernetesDiscovery)
	ret.KubernetesResources = make(map[string]*k8sconv.KubernetesResource)
	ret.UIResources = make(map[string]*v1alpha1.UIResource)
	ret.ConfigMaps = make(map[string]*v1alpha1.ConfigMap)
	ret.LiveUpdates = make(map[string]*v1alpha1.LiveUpdate)
	ret.Clusters = make(map[string]*v1alpha1.Cluster)
	ret.UIButtons = make(map[string]*v1alpha1.UIButton)
	ret.ImageMaps = make(map[string]*v1alpha1.ImageMap)
	ret.DockerImages = make(map[string]*v1alpha1.DockerImage)
	ret.CmdImages = make(map[string]*v1alpha1.CmdImage)

	return ret
}

func NewManifestState(m model.Manifest) *ManifestState {
	mn := m.Name
	ms := &ManifestState{
		Name:          mn,
		BuildStatuses: make(map[model.TargetID]*BuildStatus),
		DisableState:  v1alpha1.DisableStatePending,
		CurrentBuilds: make(map[string]model.BuildRecord),
	}

	if m.IsK8s() {
		ms.RuntimeState = NewK8sRuntimeState(m)
	} else if m.IsLocal() {
		ms.RuntimeState = LocalRuntimeState{}
	}

	// For historical reasons, DC state is initialized differently.

	return ms
}

func (ms *ManifestState) TargetID() model.TargetID {
	return ms.Name.TargetID()
}

// Returns a copy of the build status. Any changes will not change the
// engine. Allows for multiple simultaneous readers.
func (ms *ManifestState) BuildStatus(id model.TargetID) *BuildStatus {
	result, ok := ms.BuildStatuses[id]
	if !ok {
		return &BuildStatus{}
	}
	return result
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

func (ms *ManifestState) IsK8s() bool {
	_, ok := ms.RuntimeState.(K8sRuntimeState)
	return ok
}

func (ms *ManifestState) LocalRuntimeState() LocalRuntimeState {
	ret, _ := ms.RuntimeState.(LocalRuntimeState)
	return ret
}

// Return the current build that started first.
func (ms *ManifestState) EarliestCurrentBuild() model.BuildRecord {
	best := model.BuildRecord{}
	bestKey := ""
	for k, v := range ms.CurrentBuilds {
		if best.StartTime.IsZero() || best.StartTime.After(v.StartTime) || (best.StartTime == v.StartTime && k < bestKey) {
			best = v
			bestKey = k
		}
	}
	return best
}

func (ms *ManifestState) IsBuilding() bool {
	return len(ms.CurrentBuilds) != 0
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
	return ms.IsBuilding() || len(ms.BuildHistory) > 0
}

func (ms *ManifestState) MostRecentPod() v1alpha1.Pod {
	return ms.K8sRuntimeState().MostRecentPod()
}

func (ms *ManifestState) PodWithID(pid k8s.PodID) (*v1alpha1.Pod, bool) {
	name := string(pid)
	for _, pod := range ms.K8sRuntimeState().GetPods() {
		if name == pod.Name {
			return &pod, true
		}
	}
	return nil, false
}

func (ms *ManifestState) AddPendingFileChange(targetID model.TargetID, file string, timestamp time.Time) {
	if ms.IsBuilding() {
		build := ms.EarliestCurrentBuild()
		if timestamp.Before(build.StartTime) {
			// this file change occurred before the build started, but if the current build already knows
			// about it (from another target or rapid successive changes that weren't de-duped), it can be ignored
			for _, edit := range build.Edits {
				if edit == file {
					return
				}
			}
		}
		// NOTE(nick): BuildController uses these timestamps to determine which files
		// to clear after a build. In particular, it:
		//
		// 1) Grabs the pending files
		// 2) Runs a live update
		// 3) Clears the pending files with timestamps before the live update started.
		//
		// Here's the race condition: suppose a file changes, but it doesn't get into
		// the EngineState until after step (2). That means step (3) will clear the file
		// even though it wasn't live-updated properly. Because as far as we can tell,
		// the file must have been in the EngineState before the build started.
		//
		// Ideally, BuildController should be do more bookkeeping to keep track of
		// which files it consumed from which FileWatches. But we're changing
		// this architecture anyway. For now, we record the time it got into
		// the EngineState, rather than the time it was originally changed.
		//
		// This will all go away as we move things into reconcilers,
		// because reconcilers do synchronous state updates.
		isReconciler := targetID.Type == model.TargetTypeConfigs
		if !isReconciler {
			timestamp = time.Now()
		}
	}

	bs := ms.MutableBuildStatus(targetID)
	bs.FileChanges[file] = timestamp
}

func (ms *ManifestState) HasPendingFileChanges() bool {
	for _, status := range ms.BuildStatuses {
		if status.HasPendingFileChanges() {
			return true
		}
	}
	return false
}

func (ms *ManifestState) HasPendingDependencyChanges() bool {
	for _, status := range ms.BuildStatuses {
		if status.HasPendingDependencyChanges() {
			return true
		}
	}
	return false
}

func (mt *ManifestTarget) NextBuildReason() model.BuildReason {
	state := mt.State
	reason := state.TriggerReason
	if mt.State.HasPendingFileChanges() {
		reason = reason.With(model.BuildReasonFlagChangedFiles)
	}
	if mt.State.HasPendingDependencyChanges() {
		reason = reason.With(model.BuildReasonFlagChangedDeps)
	}
	if !mt.State.PendingManifestChange.IsZero() {
		reason = reason.With(model.BuildReasonFlagConfig)
	}
	if !mt.State.StartedFirstBuild() && mt.Manifest.TriggerMode.AutoInitial() {
		reason = reason.With(model.BuildReasonFlagInit)
	}
	return reason
}

// Whether changes have been made to this Manifest's synced files
// or config since the last build.
//
// Returns:
// bool: whether changes have been made
// Time: the time of the earliest change
func (ms *ManifestState) HasPendingChanges() (bool, time.Time) {
	return ms.HasPendingChangesBeforeOrEqual(time.Now())
}

// Like HasPendingChanges, but relative to a particular time.
func (ms *ManifestState) HasPendingChangesBeforeOrEqual(highWaterMark time.Time) (bool, time.Time) {
	ok := false
	earliest := highWaterMark
	t := ms.PendingManifestChange
	if !t.IsZero() && timecmp.BeforeOrEqual(t, earliest) {
		ok = true
		earliest = t
	}

	for _, status := range ms.BuildStatuses {
		for _, t := range status.PendingFileChanges() {
			if !t.IsZero() && timecmp.BeforeOrEqual(t, earliest) {
				ok = true
				earliest = t
			}
		}

		for _, t := range status.PendingDependencyChanges() {
			if !t.IsZero() && timecmp.BeforeOrEqual(t, earliest) {
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

func (ms *ManifestState) UpdateStatus(triggerMode model.TriggerMode) v1alpha1.UpdateStatus {
	currentBuild := ms.EarliestCurrentBuild()
	hasPendingChanges, _ := ms.HasPendingChanges()
	lastBuild := ms.LastBuild()
	lastBuildError := lastBuild.Error != nil
	hasPendingBuild := false
	if ms.TriggerReason != 0 {
		hasPendingBuild = true
	} else if triggerMode.AutoOnChange() && hasPendingChanges {
		hasPendingBuild = true
	} else if triggerMode.AutoInitial() && currentBuild.Empty() && lastBuild.Empty() {
		hasPendingBuild = true
	}

	if !currentBuild.Empty() {
		return v1alpha1.UpdateStatusInProgress
	} else if hasPendingBuild {
		return v1alpha1.UpdateStatusPending
	} else if lastBuildError {
		return v1alpha1.UpdateStatusError
	} else if !lastBuild.Empty() {
		return v1alpha1.UpdateStatusOK
	}
	return v1alpha1.UpdateStatusNone
}

// Check the runtime status of the individual status fields.
//
// The individual status fields don't know anything about how resources are
// triggered (i.e., whether they're waiting on a dependent resource to build or
// a manual trigger). So we need to consider that information here.
func (ms *ManifestState) RuntimeStatus(triggerMode model.TriggerMode) v1alpha1.RuntimeStatus {
	runStatus := v1alpha1.RuntimeStatusUnknown
	if ms.RuntimeState != nil {
		runStatus = ms.RuntimeState.RuntimeStatus()
	}

	if runStatus == v1alpha1.RuntimeStatusPending || runStatus == v1alpha1.RuntimeStatusUnknown {
		// Let's just borrow the trigger analysis logic from UpdateStatus().
		updateStatus := ms.UpdateStatus(triggerMode)
		if updateStatus == v1alpha1.UpdateStatusNone {
			runStatus = v1alpha1.RuntimeStatusNone
		} else if updateStatus == v1alpha1.UpdateStatusPending || updateStatus == v1alpha1.UpdateStatusInProgress {
			runStatus = v1alpha1.RuntimeStatusPending
		}
	}
	return runStatus
}

var _ model.TargetStatus = &ManifestState{}

func ManifestTargetEndpoints(mt *ManifestTarget) (endpoints []model.Link) {
	if mt.Manifest.IsK8s() {
		k8sTarg := mt.Manifest.K8sTarget()
		endpoints = append(endpoints, k8sTarg.Links...)

		// If the user specified port-forwards in the Tiltfile, we
		// assume that's what they want to see in the UI (so it
		// takes precedence over any load balancer URLs
		portForwardSpec := k8sTarg.PortForwardTemplateSpec
		if portForwardSpec != nil && len(portForwardSpec.Forwards) > 0 {
			for _, pf := range portForwardSpec.Forwards {
				endpoints = append(endpoints, model.PortForwardToLink(pf))
			}
			return endpoints
		}

		lbEndpoints := []model.Link{}
		for _, u := range mt.State.K8sRuntimeState().LBs {
			if u != nil {
				lbEndpoints = append(lbEndpoints, model.Link{URL: u})
			}
		}
		// Sort so the ordering of LB endpoints is deterministic
		// (otherwise it's not, because they live in a map)
		sort.Sort(model.ByURL(lbEndpoints))
		endpoints = append(endpoints, lbEndpoints...)
	}

	localResourceLinks := mt.Manifest.LocalTarget().Links
	if len(localResourceLinks) > 0 {
		return localResourceLinks
	}

	if mt.Manifest.IsDC() {
		hostPorts := make(map[int32]bool)
		publishedPorts := mt.Manifest.DockerComposeTarget().PublishedPorts()
		inferLinks := mt.Manifest.DockerComposeTarget().InferLinks()
		for _, p := range publishedPorts {
			if p == 0 || hostPorts[int32(p)] {
				continue
			}
			hostPorts[int32(p)] = true
			if inferLinks {
				endpoints = append(endpoints, model.MustNewLink(fmt.Sprintf("http://localhost:%d/", p), ""))
			}
		}

		for _, binding := range mt.State.DCRuntimeState().Ports {
			// Docker usually contains multiple bindings for each port - one for ipv4 (0.0.0.0)
			// and one for ipv6 (::1).
			p := binding.HostPort
			if hostPorts[p] {
				continue
			}
			hostPorts[p] = true
			if inferLinks {
				endpoints = append(endpoints, model.MustNewLink(fmt.Sprintf("http://localhost:%d/", p), ""))
			}
		}

		endpoints = append(endpoints, mt.Manifest.DockerComposeTarget().Links...)
	}

	return endpoints
}

const MainTiltfileManifestName = model.MainTiltfileManifestName
