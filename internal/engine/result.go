package engine

import (
	"sort"

	"github.com/docker/distribution/reference"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/ospath"
)

// The results of a successful build.
type BuildResult struct {
	Image    reference.NamedTagged
	Entities []k8s.K8sEntity
}

func (b BuildResult) IsEmpty() bool {
	return b.Image == nil
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

func NewBuildState(result BuildResult) BuildState {
	return BuildState{
		LastResult:      result,
		filesChangedSet: make(map[string]bool, 0),
	}
}

func (b BuildState) LastImage() reference.NamedTagged {
	return b.LastResult.Image
}

// Return the files changed in sorted order.
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

var BuildStateClean = BuildState{}
