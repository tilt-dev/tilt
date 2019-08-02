package store

import (
	"log"
	"sort"

	"github.com/docker/distribution/reference"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
)

// The results of a successful build.
type BuildResult struct {
	// The image target that this built.
	TargetID model.TargetID

	// The name+tag of the image that the pod is running.
	//
	// The tag is derived from a content-addressable digest.
	Image reference.NamedTagged

	// The ID of the container that Docker Compose created.
	//
	// When we deploy a Docker Compose service, we wait synchronously for the
	// container to start. Note that this is a different concurrency model than
	// we use for Kubernetes, where the pods appear some time later via an
	// asynchronous event.
	DockerComposeContainerID container.ID

	// The ID of the container(s) that we live-updated in-place.
	//
	// The contents of the container have diverged from the image it's built on,
	// so we need to keep track of that.
	LiveUpdatedContainerIDs []container.ID
}

// For in-place container updates.
func NewLiveUpdateBuildResult(id model.TargetID, containerIDs []container.ID) BuildResult {
	return BuildResult{
		TargetID:                id,
		LiveUpdatedContainerIDs: containerIDs,
	}
}

// For image targets.
func NewImageBuildResult(id model.TargetID, image reference.NamedTagged) BuildResult {
	return BuildResult{
		TargetID: id,
		Image:    image,
	}
}

// For docker compose deploy targets.
func NewDockerComposeDeployResult(id model.TargetID, containerID container.ID) BuildResult {
	return BuildResult{
		TargetID:                 id,
		DockerComposeContainerID: containerID,
	}
}

func (b BuildResult) IsEmpty() bool {
	return b.TargetID.Empty()
}

func (b BuildResult) HasImage() bool {
	return b.Image != nil
}

func (b BuildResult) IsInPlaceUpdate() bool {
	return len(b.LiveUpdatedContainerIDs) != 0
}

type BuildResultSet map[model.TargetID]BuildResult

func (set BuildResultSet) LiveUpdatedContainerIDs() []container.ID {
	result := []container.ID{}
	for _, r := range set {
		result = append(result, r.LiveUpdatedContainerIDs...)
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

// Returns a container ID iff it's the only container ID in the result set.
// If there are multiple container IDs, we have to give up.
func (set BuildResultSet) OneAndOnlyLiveUpdatedContainerID() container.ID {
	var id container.ID
	for _, result := range set {
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

// The state of the system since the last successful build.
// This data structure should be considered immutable.
// All methods that return a new BuildState should first clone the existing build state.
type BuildState struct {
	// The last successful build.
	LastResult BuildResult

	// Files changed since the last result was build.
	// This must be liberal: it's ok if this has too many files, but not ok if it has too few.
	FilesChangedSet map[string]bool

	RunningContainers []ContainerInfo
}

func NewBuildState(result BuildResult, files []string) BuildState {
	set := make(map[string]bool, len(files))
	for _, f := range files {
		set[f] = true
	}
	return BuildState{
		LastResult:      result,
		FilesChangedSet: set,
	}
}

func (b BuildState) WithRunningContainers(cInfos []ContainerInfo) BuildState {
	b.RunningContainers = cInfos
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
func (b BuildState) LastImageAsString() string {
	img := b.LastResult.Image
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
	for file, _ := range b.FilesChangedSet {
		result = append(result, file)
	}
	sort.Strings(result)
	return result
}

// A build state is empty if there are no previous results.
func (b BuildState) IsEmpty() bool {
	return b.LastResult.IsEmpty()
}

func (b BuildState) HasImage() bool {
	return b.LastResult.HasImage()
}

// Whether the image represented by this state needs to be built.
// If the image has already been built, and no files have been
// changed since then, then we can re-use the previous result.
func (b BuildState) NeedsImageBuild() bool {
	lastBuildWasImgBuild := b.LastResult.HasImage() && !b.LastResult.IsInPlaceUpdate()
	return !lastBuildWasImgBuild || len(b.FilesChangedSet) > 0
}

type BuildStateSet map[model.TargetID]BuildState

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
		return RunningContainersForDC(mt.State.DCResourceState())
	}

	result := []ContainerInfo{}
	for _, iTarget := range mt.Manifest.ImageTargets {
		result = append(result, RunningContainersForTarget(iTarget, mt.State.DeployID, mt.State.PodSet)...)
	}
	return result
}

// If all containers running the given image are ready, returns info for them.
// (Currently only supports containers running on a single pod.)
func RunningContainersForTarget(iTarget model.ImageTarget, deployID model.DeployID, podSet PodSet) []ContainerInfo {
	if podSet.Len() != 1 {
		return nil
	}

	pod := podSet.MostRecentPod()
	if pod.PodID == "" {
		return nil
	}

	if podSet.DeployID != deployID {
		return nil
	}

	var containers []ContainerInfo
	for _, c := range pod.Containers {
		// Only return containers matching our image
		if c.ImageRef == nil || iTarget.DeploymentRef.Name() != c.ImageRef.Name() {
			log.Println("SKIPPING", iTarget.DeploymentRef, c.ImageRef)
			continue
		}
		if c.ID == "" || c.Name == "" || !c.Ready {
			// If we're missing any relevant info for this container, OR if the
			// container isn't ready, we can't update it in place.
			// (Since we'll need to fully rebuild this image, we shouldn't bother
			// in-place updating ANY containers on this pod -- they'll all
			// be recreated when we image build. So don't return ANY ContainerInfos.)
			return nil
		}
		containers = append(containers, ContainerInfo{
			PodID:         pod.PodID,
			ContainerID:   c.ID,
			ContainerName: c.Name,
			Namespace:     pod.Namespace,
		})
	}

	return containers
}

func RunningContainersForDC(state dockercompose.State) []ContainerInfo {
	return []ContainerInfo{
		ContainerInfo{ContainerID: state.ContainerID},
	}
}

var BuildStateClean = BuildState{}
