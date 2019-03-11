package store

import (
	"fmt"
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

	// Some of our build engines replace the files in-place, rather
	// than building a new image. This captures how much the code
	// running on-pod has diverged from the original image.
	FilesReplacedSet map[string]bool
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

// Clone the build result and add new replaced files.
// Does not do a deep clone of the underlying entities.
func (b BuildResult) ShallowCloneForContainerUpdate(filesReplacedSet map[string]bool) BuildResult {
	result := BuildResult{}
	result.TargetID = b.TargetID
	result.Image = b.Image

	newSet := make(map[string]bool, len(b.FilesReplacedSet)+len(filesReplacedSet))
	for k, v := range b.FilesReplacedSet {
		newSet[k] = v
	}
	for k, v := range filesReplacedSet {
		newSet[k] = v
	}
	result.FilesReplacedSet = newSet
	return result
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

	DeployInfo DeployInfo
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

func (b BuildState) WithDeployTarget(d DeployInfo) BuildState {
	b.DeployInfo = d
	return b
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

// Return the files changed since the last result's image in sorted order.
// The sorting helps ensure that this is deterministic, both for testing
// and for deterministic builds.
// Errors if there was no last result image.
func (b BuildState) FilesChangedSinceLastResultImage() ([]string, error) {
	if !b.LastResult.HasImage() {
		return nil, fmt.Errorf("No image in last result")
	}

	cSet := b.FilesChangedSet
	rSet := b.LastResult.FilesReplacedSet
	sum := make(map[string]bool, len(cSet)+len(rSet))
	for k, v := range cSet {
		sum[k] = v
	}
	for k, v := range rSet {
		sum[k] = v
	}

	result := make([]string, 0, len(sum))
	for file, _ := range sum {
		result = append(result, file)
	}
	sort.Strings(result)
	return result, nil
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
	alreadyBuilt := b.LastResult.HasImage()
	return !alreadyBuilt || len(b.FilesChangedSet) > 0
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

// The information we need to find a ready container.
type DeployInfo struct {
	PodID         k8s.PodID
	ContainerID   container.ID
	ContainerName container.Name
	Namespace     k8s.Namespace
}

func (d DeployInfo) Empty() bool {
	return d == DeployInfo{}
}

// Check to see if there's a single, unambiguous Ready container
// in the given PodSet. If so, create a DeployInfo for that container.
func NewDeployInfo(iTarget model.ImageTarget, podSet PodSet) DeployInfo {
	if podSet.Len() != 1 {
		return DeployInfo{}
	}

	pod := podSet.MostRecentPod()
	if pod.PodID == "" || pod.ContainerID == "" || pod.ContainerName == "" || !pod.ContainerReady {
		return DeployInfo{}
	}

	// Only return the pod if it matches our image.
	if pod.ContainerImageRef == nil || !iTarget.Ref.Matches(pod.ContainerImageRef) {
		return DeployInfo{}
	}

	return DeployInfo{
		PodID:         pod.PodID,
		ContainerID:   pod.ContainerID,
		ContainerName: pod.ContainerName,
		Namespace:     pod.Namespace,
	}
}

func NewDeployInfoFromDC(state dockercompose.State) DeployInfo {
	return DeployInfo{ContainerID: state.ContainerID}
}

var BuildStateClean = BuildState{}
