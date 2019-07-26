package store

import (
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

	// If this build was a container build, containerID we built on top of
	ContainerID container.ID
}

// For docker-compose deploys that don't have any built images.
func NewContainerBuildResult(id model.TargetID, containerID container.ID) BuildResult {
	return BuildResult{
		TargetID:    id,
		ContainerID: containerID,
	}
}

// For image targets. The container id will be added later.
func NewImageBuildResult(id model.TargetID, image reference.NamedTagged) BuildResult {
	return BuildResult{
		TargetID: id,
		Image:    image,
	}
}

func (b BuildResult) IsEmpty() bool {
	return b.TargetID.Empty()
}

func (b BuildResult) HasImage() bool {
	return b.Image != nil
}

func (b BuildResult) IsInPlaceUpdate() bool {
	return !b.ContainerID.Empty()
}

type BuildResultSet map[model.TargetID]BuildResult

// Returns a container ID iff it's the only container ID in the result set.
// If there are multiple container IDs, we have to give up.
func (set BuildResultSet) OneAndOnlyContainerID() container.ID {
	var id container.ID
	for _, result := range set {
		if result.ContainerID == "" {
			continue
		}

		if id != "" && result.ContainerID != id {
			return ""
		}

		id = result.ContainerID
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

// Check to see if there's a single, unambiguous Ready container
// in the given PodSet. If so, create a ContainerInfo for that container.
// TODO(maia): soon will return cInfos for ALL ready containers running this image
func RunningContainersForTarget(iTarget model.ImageTarget, deployID model.DeployID, podSet PodSet) []ContainerInfo {
	if podSet.Len() != 1 {
		return nil
	}

	pod := podSet.MostRecentPod()
	if pod.PodID == "" || pod.ContainerID() == "" || pod.ContainerName() == "" || !pod.ContainerReady() {
		return nil
	}

	if podSet.DeployID != deployID {
		return nil
	}

	// Only return the pod if it matches our image.
	if pod.ContainerImageRef() == nil || iTarget.DeploymentRef.Name() != pod.ContainerImageRef().Name() {
		return nil
	}

	return []ContainerInfo{
		ContainerInfo{
			PodID:         pod.PodID,
			ContainerID:   pod.ContainerID(),
			ContainerName: pod.ContainerName(),
			Namespace:     pod.Namespace,
		},
	}
}

func RunningContainersForDC(state dockercompose.State) []ContainerInfo {
	return []ContainerInfo{
		ContainerInfo{ContainerID: state.ContainerID},
	}
}

var BuildStateClean = BuildState{}
