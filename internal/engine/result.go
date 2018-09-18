package engine

import (
	"fmt"
	"sort"

	"github.com/docker/distribution/reference"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
)

// The results of a successful build.
type BuildResult struct {
	// The name+tag of the image that the pod is running.
	//
	// The tag is derived from a content-addressable digest.
	Image reference.NamedTagged

	// The k8s entities deployed alongside the image.
	Entities []k8s.K8sEntity

	// The container ID for the deployed image.
	Container k8s.ContainerID

	// Some of our build engines replace the files in-place, rather
	// than building a new image. This captures how much the code
	// running on-pod has diverged from the original image.
	FilesReplacedSet map[string]bool
}

func (b BuildResult) IsEmpty() bool {
	return b.Image == nil && b.Container == ""
}

func (b BuildResult) HasImage() bool {
	return b.Image != nil
}

func (b BuildResult) HasContainer() bool {
	return b.Container != ""
}

// Clone the build result and add new replaced files.
// Does not do a deep clone of the underlying entities.
func (b BuildResult) ShallowCloneForContainerUpdate(cID k8s.ContainerID, filesReplacedSet map[string]bool) BuildResult {
	result := BuildResult{}
	result.Image = b.Image
	result.Entities = append([]k8s.K8sEntity{}, b.Entities...)
	result.Container = cID

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

// The state of the system since the last successful build.
// This data structure should be considered immutable.
// All methods that return a new BuildState should first clone the existing build state.
type BuildState struct {
	// The last successful build.
	LastResult BuildResult

	// Files changed since the last result was build.
	// This must be liberal: it's ok if this has too many files, but not ok if it has too few.
	filesChangedSet map[string]bool
}

type BuildStatesByName map[model.ManifestName]BuildState

func NewBuildState(result BuildResult) BuildState {
	return BuildState{
		LastResult:      result,
		filesChangedSet: make(map[string]bool, 0),
	}
}

func (b BuildState) LastImage() reference.NamedTagged {
	return b.LastResult.Image
}

// Return the files changed since the last result in sorted order.
// The sorting helps ensure that this is deterministic, both for testing
// and for deterministic builds.
func (b BuildState) FilesChanged() []string {
	result := make([]string, 0, len(b.filesChangedSet))
	for file, _ := range b.filesChangedSet {
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

	cSet := b.filesChangedSet
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

func (b BuildState) NewStateWithFilesChanged(files []string) BuildState {
	result := NewBuildState(b.LastResult)
	for k, v := range b.filesChangedSet {
		result.filesChangedSet[k] = v
	}
	for _, f := range files {
		result.filesChangedSet[f] = true
	}
	return result
}

// Check if the filesChangedSet only contains spurious changes that
// we don't want to rebuild on, like IDE temp/lock files.
//
// NOTE(nick): This isn't an ideal solution. In an ideal world, the user would
// put everything to ignore in their gitignore/dockerignore files. This is a stop-gap
// so they don't have a terrible experience if those files aren't there or
// aren't in the right places.
func (b BuildState) OnlySpuriousChanges() (bool, error) {
	// If a lot of files have changed, don't treat this as spurious.
	if len(b.filesChangedSet) > 3 {
		return false, nil
	}

	for f := range b.filesChangedSet {
		broken, err := ospath.IsBrokenSymlink(f)
		if err != nil {
			return false, err
		}

		if !broken {
			return false, nil
		}
	}
	return true, nil
}

// A build state is empty if there are no previous results.
func (b BuildState) IsEmpty() bool {
	return b.LastResult.IsEmpty()
}

func (b BuildState) HasImage() bool {
	return b.LastResult.HasImage()
}

var BuildStateClean = BuildState{}
