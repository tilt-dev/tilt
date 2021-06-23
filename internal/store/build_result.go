package store

import (
	"fmt"
	"sort"

	v1 "k8s.io/api/core/v1"

	"github.com/docker/distribution/reference"
	dockertypes "github.com/docker/docker/api/types"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/dockercompose"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// The results of a build.
//
// If a build is successful, the builder should always return a BuildResult to
// indicate the output artifacts of the build (e.g., any new images).
//
// If a build is not successful, things get messier. Certain types of failure
// may still return a result (e.g., a failed live update might return
// the container IDs where the error happened).
//
// Long-term, we want this interface to be more like Bazel's SpawnResult
// https://github.com/bazelbuild/bazel/blob/master/src/main/java/com/google/devtools/build/lib/actions/SpawnResult.java#L36
// where a builder always returns a Result, but with a code for each
// of the different failure types.
type BuildResult interface {
	TargetID() model.TargetID
	BuildType() model.BuildType
}

type LocalBuildResult struct {
	id model.TargetID
}

func (r LocalBuildResult) TargetID() model.TargetID   { return r.id }
func (r LocalBuildResult) BuildType() model.BuildType { return model.BuildTypeLocal }

func NewLocalBuildResult(id model.TargetID) LocalBuildResult {
	return LocalBuildResult{
		id: id,
	}
}

type ImageBuildResult struct {
	id model.TargetID

	// TODO(maia): it would make the most sense for the ImageBuildResult to know what it BUILT, and for us
	//   to calculate the ClusterRef (if different from LocalRef) when we have to inject it, but
	//   storing all the info on ImageBuildResult for now was the fastest/safest way to ship this.
	// Note: image tag is derived from a content-addressable digest.
	ImageLocalRef   reference.NamedTagged // built image, as referenced from outside the cluster (in Dockerfile, docker push etc.)
	ImageClusterRef reference.NamedTagged // built image, as referenced from the cluster (in K8s YAML, etc.)
	// Often ImageLocalRef and ImageClusterRef will be the same, but may diverge: e.g.
	// when using KIND + local registry, localRef is localhost:1234/my-img:tilt-abc,
	// ClusterRef is http://registry/my-img:tilt-abc

	ImageMapStatus v1alpha1.ImageMapStatus
}

func (r ImageBuildResult) TargetID() model.TargetID   { return r.id }
func (r ImageBuildResult) BuildType() model.BuildType { return model.BuildTypeImage }

// For image targets.
func NewImageBuildResult(id model.TargetID, localRef, clusterRef reference.NamedTagged) ImageBuildResult {
	return ImageBuildResult{
		id:              id,
		ImageLocalRef:   localRef,
		ImageClusterRef: clusterRef,
		ImageMapStatus: v1alpha1.ImageMapStatus{
			Image: clusterRef.String(),
		},
	}
}

// When localRef == ClusterRef
func NewImageBuildResultSingleRef(id model.TargetID, ref reference.NamedTagged) ImageBuildResult {
	return NewImageBuildResult(id, ref, ref)
}

type LiveUpdateBuildResult struct {
	id model.TargetID

	// The ID of the container(s) that we live-updated in-place.
	//
	// The contents of the container have diverged from the image it's built on,
	// so we need to keep track of that.
	LiveUpdatedContainerIDs []container.ID
}

func (r LiveUpdateBuildResult) TargetID() model.TargetID   { return r.id }
func (r LiveUpdateBuildResult) BuildType() model.BuildType { return model.BuildTypeLiveUpdate }

// For in-place container updates.
func NewLiveUpdateBuildResult(id model.TargetID, containerIDs []container.ID) LiveUpdateBuildResult {
	return LiveUpdateBuildResult{
		id:                      id,
		LiveUpdatedContainerIDs: containerIDs,
	}
}

type DockerComposeBuildResult struct {
	id model.TargetID

	// The ID of the container that Docker Compose created.
	//
	// When we deploy a Docker Compose service, we wait synchronously for the
	// container to start. Note that this is a different concurrency model than
	// we use for Kubernetes, where the pods appear some time later via an
	// asynchronous event.
	DockerComposeContainerID container.ID

	// The initial state of the container.
	ContainerState *dockertypes.ContainerState
}

func (r DockerComposeBuildResult) TargetID() model.TargetID   { return r.id }
func (r DockerComposeBuildResult) BuildType() model.BuildType { return model.BuildTypeDockerCompose }

// For docker compose deploy targets.
func NewDockerComposeDeployResult(id model.TargetID, containerID container.ID, state *dockertypes.ContainerState) DockerComposeBuildResult {
	return DockerComposeBuildResult{
		id:                       id,
		DockerComposeContainerID: containerID,
		ContainerState:           state,
	}
}

type K8sBuildResult struct {
	id model.TargetID

	// DeployedEntities are references to the objects that we deployed to a Kubernetes cluster.
	DeployedEntities []v1.ObjectReference

	// Hashes of the pod template specs that we deployed to a Kubernetes cluster.
	PodTemplateSpecHashes []k8s.PodTemplateSpecHash

	AppliedEntitiesText string
}

func (r K8sBuildResult) TargetID() model.TargetID   { return r.id }
func (r K8sBuildResult) BuildType() model.BuildType { return model.BuildTypeK8s }

// NewK8sDeployResult creates a deploy result for Kubernetes deploy targets.
func NewK8sDeployResult(id model.TargetID, hashes []k8s.PodTemplateSpecHash, appliedEntities []k8s.K8sEntity) BuildResult {
	// Remove verbose fields from the YAML.
	for _, e := range appliedEntities {
		e.Clean()
	}

	appliedEntitiesText, err := k8s.SerializeSpecYAML(appliedEntities)
	if err != nil {
		appliedEntitiesText = fmt.Sprintf("unable to serialize entities to yaml: %s", err.Error())
	}

	deployedRefs := make(k8s.ObjRefList, len(appliedEntities))
	for i, entity := range appliedEntities {
		deployedRefs[i] = entity.ToObjectReference()
	}

	return K8sBuildResult{
		id:                    id,
		DeployedEntities:      deployedRefs,
		PodTemplateSpecHashes: hashes,
		AppliedEntitiesText:   appliedEntitiesText,
	}
}

func LocalImageRefFromBuildResult(r BuildResult) reference.NamedTagged {
	switch r := r.(type) {
	case ImageBuildResult:
		return r.ImageLocalRef
	}
	return nil
}

func ClusterImageRefFromBuildResult(r BuildResult) reference.NamedTagged {
	switch r := r.(type) {
	case ImageBuildResult:
		return r.ImageClusterRef
	}
	return nil
}

type BuildResultSet map[model.TargetID]BuildResult

func (set BuildResultSet) LiveUpdatedContainerIDs() []container.ID {
	result := []container.ID{}
	for _, r := range set {
		r, ok := r.(LiveUpdateBuildResult)
		if ok {
			result = append(result, r.LiveUpdatedContainerIDs...)
		}
	}
	return result
}

func (set BuildResultSet) DeployedEntities() k8s.ObjRefList {
	var result k8s.ObjRefList
	for _, r := range set {
		r, ok := r.(K8sBuildResult)
		if ok {
			for _, ref := range r.DeployedEntities {
				// shallow copy is fine; there's no reference types on v1.ObjectReference
				result = append(result, ref)
			}
		}
	}
	return result
}

func (set BuildResultSet) DeployedPodTemplateSpecHashes() PodTemplateSpecHashSet {
	result := NewPodTemplateSpecHashSet()
	for _, r := range set {
		r, ok := r.(K8sBuildResult)
		if ok {
			result.Add(r.PodTemplateSpecHashes...)
		}
	}
	return result
}

func MergeBuildResultsSet(a, b BuildResultSet) BuildResultSet {
	res := make(BuildResultSet)
	for k, v := range a {
		res[k] = v
	}
	for k, v := range b {
		res[k] = v
	}
	return res
}

func (set BuildResultSet) BuildTypes() []model.BuildType {
	btMap := make(map[model.BuildType]bool, len(set))
	for _, br := range set {
		if br != nil {
			btMap[br.BuildType()] = true
		}
	}
	result := make([]model.BuildType, 0, len(btMap))
	for key := range btMap {
		result = append(result, key)
	}
	return result
}

// Returns a container ID iff it's the only container ID in the result set.
// If there are multiple container IDs, we have to give up.
func (set BuildResultSet) OneAndOnlyLiveUpdatedContainerID() container.ID {
	var id container.ID
	for _, br := range set {
		result, ok := br.(LiveUpdateBuildResult)
		if !ok {
			continue
		}

		if len(result.LiveUpdatedContainerIDs) == 0 {
			continue
		}

		if len(result.LiveUpdatedContainerIDs) > 1 {
			return ""
		}

		curID := result.LiveUpdatedContainerIDs[0]
		if curID == "" {
			continue
		}

		if id != "" && curID != id {
			return ""
		}

		id = curID
	}
	return id
}

// A BuildResultSet that can only hold image build results.
type ImageBuildResultSet map[model.TargetID]ImageBuildResult

func (s ImageBuildResultSet) ToBuildResultSet() BuildResultSet {
	result := BuildResultSet{}
	for k, v := range s {
		result[k] = v
	}
	return result
}

// The state of the system since the last successful build.
// This data structure should be considered immutable.
// All methods that return a new BuildState should first clone the existing build state.
type BuildState struct {
	// The last result.
	LastResult BuildResult

	// Files changed since the last result was build.
	// This must be liberal: it's ok if this has too many files, but not ok if it has too few.
	FilesChangedSet map[string]bool

	// Dependencies changed since the last result was built
	DepsChangedSet map[model.TargetID]bool

	// There are three kinds of triggers:
	//
	// 1) If a resource is in trigger_mode=TRIGGER_MODE_AUTO, then the resource auto-builds.
	//    Pressing the trigger will always do a full image build.
	//
	// 2) If a resource is in trigger_mode=TRIGGER_MODE_MANUAL and there are no pending changes,
	//    then pressing the trigger will do a full image build.
	//
	// 3) If a resource is in trigger_mode=TRIGGER_MODE_MANUAL, and there are
	//    pending changes, then pressing the trigger will do a live_update (if one
	//    is configured; otherwise, will do an image build as normal)
	//
	// This field indicates case 1 || case 2 -- i.e. that we should skip
	// live_update, and force an image build (even if there are no changed files)
	FullBuildTriggered bool

	RunningContainers []ContainerInfo

	// If we had an error retrieving running containers
	RunningContainerError error
}

func NewBuildState(result BuildResult, files []string, pendingDeps []model.TargetID) BuildState {
	set := make(map[string]bool, len(files))
	for _, f := range files {
		set[f] = true
	}
	depsSet := make(map[model.TargetID]bool, len(pendingDeps))
	for _, d := range pendingDeps {
		depsSet[d] = true
	}
	return BuildState{
		LastResult:      result,
		FilesChangedSet: set,
		DepsChangedSet:  depsSet,
	}
}

func (b BuildState) WithRunningContainers(cInfos []ContainerInfo) BuildState {
	b.RunningContainers = cInfos
	return b
}

func (b BuildState) WithRunningContainerError(err error) BuildState {
	b.RunningContainerError = err
	return b
}

func (b BuildState) WithFullBuildTriggered(isImageBuildTrigger bool) BuildState {
	b.FullBuildTriggered = isImageBuildTrigger
	return b
}

// NOTE(maia): Interim method to replicate old behavior where every
// BuildState had a single ContainerInfo
func (b BuildState) OneContainerInfo() ContainerInfo {
	if len(b.RunningContainers) == 0 {
		return ContainerInfo{}
	}
	return b.RunningContainers[0]
}
func (b BuildState) LastLocalImageAsString() string {
	img := LocalImageRefFromBuildResult(b.LastResult)
	if img == nil {
		return ""
	}
	return img.String()
}

// Return the files changed since the last result in sorted order.
// The sorting helps ensure that this is deterministic, both for testing
// and for deterministic builds.
func (b BuildState) FilesChanged() []string {
	result := make([]string, 0, len(b.FilesChangedSet))
	for file := range b.FilesChangedSet {
		result = append(result, file)
	}
	sort.Strings(result)
	return result
}

// A build state is empty if there are no previous results.
func (b BuildState) IsEmpty() bool {
	return b.LastResult == nil
}

func (b BuildState) HasLastResult() bool {
	return b.LastResult != nil
}

// Whether the image represented by this state needs to be built.
// If the image has already been built, and no files have been
// changed since then, then we can re-use the previous result.
func (b BuildState) NeedsImageBuild() bool {
	lastBuildWasImgBuild := b.LastResult != nil &&
		b.LastResult.BuildType() == model.BuildTypeImage
	return !lastBuildWasImgBuild ||
		len(b.FilesChangedSet) > 0 ||
		len(b.DepsChangedSet) > 0 ||
		b.FullBuildTriggered
}

type BuildStateSet map[model.TargetID]BuildState

func (set BuildStateSet) FullBuildTriggered() bool {
	for _, state := range set {
		if state.FullBuildTriggered {
			return true
		}
	}
	return false
}

func (set BuildStateSet) Empty() bool {
	return len(set) == 0
}

func (set BuildStateSet) FilesChanged() []string {
	resultMap := map[string]bool{}
	for _, state := range set {
		for k := range state.FilesChangedSet {
			resultMap[k] = true
		}
	}

	result := make([]string, 0, len(resultMap))
	for k := range resultMap {
		result = append(result, k)
	}
	sort.Strings(result)
	return result
}

// Information describing a single running & ready container
type ContainerInfo struct {
	PodID         k8s.PodID
	ContainerID   container.ID
	ContainerName container.Name
	Namespace     k8s.Namespace
}

func (c ContainerInfo) Empty() bool {
	return c == ContainerInfo{}
}

func IDsForInfos(infos []ContainerInfo) []container.ID {
	ids := make([]container.ID, len(infos))
	for i, info := range infos {
		ids[i] = info.ContainerID
	}
	return ids
}

func AllRunningContainers(mt *ManifestTarget) []ContainerInfo {
	if mt.Manifest.IsDC() {
		return RunningContainersForDC(mt.State.DCRuntimeState())
	}

	var result []ContainerInfo
	for _, iTarget := range mt.Manifest.ImageTargets {
		cInfos, err := RunningContainersForTargetForOnePod(iTarget, mt.State.K8sRuntimeState())
		if err != nil {
			// HACK(maia): just don't collect container info for targets running
			// more than one pod -- we don't support LiveUpdating them anyway,
			// so no need to monitor those containers for crashes.
			continue
		}
		result = append(result, cInfos...)
	}
	return result
}

// If all containers running the given image are ready, returns info for them.
// (If this image is running on multiple pods, return an error.)
func RunningContainersForTargetForOnePod(iTarget model.ImageTarget, runtimeState K8sRuntimeState) ([]ContainerInfo, error) {
	if runtimeState.PodLen() > 1 {
		return nil, fmt.Errorf("can only get container info for a single pod; image target %s has %d pods", iTarget.ID(), runtimeState.PodLen())
	}

	if runtimeState.PodLen() == 0 {
		return nil, nil
	}

	pod := runtimeState.MostRecentPod()
	if pod.Name == "" {
		return nil, nil
	}

	// If there was a recent deploy, the runtime state might not have the
	// new pods yet. We check the PodAncestorID and see if it's in the most
	// recent deploy set. If it's not, then we can should ignore these pods.
	ancestorUID := runtimeState.PodAncestorUID
	if ancestorUID != "" && !runtimeState.DeployedUIDSet().Contains(ancestorUID) {
		return nil, nil
	}

	var containers []ContainerInfo
	for _, c := range pod.Containers {
		// Only return containers matching our image
		imageRef, err := container.ParseNamed(c.Image)
		if err != nil || imageRef == nil || iTarget.Refs.ClusterRef().Name() != imageRef.Name() {
			continue
		}
		if c.ID == "" || c.Name == "" || c.State.Running == nil {
			// If we're missing any relevant info for this container, OR if the
			// container isn't running, we can't update it in place.
			// (Since we'll need to fully rebuild this image, we shouldn't bother
			// in-place updating ANY containers on this pod -- they'll all
			// be recreated when we image build. So don't return ANY ContainerInfos.)
			return nil, nil
		}
		containers = append(containers, ContainerInfo{
			PodID:         k8s.PodID(pod.Name),
			ContainerID:   container.ID(c.ID),
			ContainerName: container.Name(c.Name),
			Namespace:     k8s.Namespace(pod.Namespace),
		})
	}

	return containers, nil
}

func RunningContainersForDC(state dockercompose.State) []ContainerInfo {
	return []ContainerInfo{
		ContainerInfo{ContainerID: state.ContainerID},
	}
}

var BuildStateClean = BuildState{}
