package store

import (
	"sort"

	"github.com/docker/distribution/reference"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
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
	id             model.TargetID
	ImageMapStatus v1alpha1.ImageMapStatus
}

func (r ImageBuildResult) TargetID() model.TargetID   { return r.id }
func (r ImageBuildResult) BuildType() model.BuildType { return model.BuildTypeImage }

// For image targets.
func NewImageBuildResult(id model.TargetID, localRef, clusterRef reference.NamedTagged) ImageBuildResult {
	return ImageBuildResult{
		id: id,
		ImageMapStatus: v1alpha1.ImageMapStatus{
			Image:            container.FamiliarString(clusterRef),
			ImageFromCluster: container.FamiliarString(clusterRef),
			ImageFromLocal:   container.FamiliarString(localRef),
		},
	}
}

// When localRef == ClusterRef
func NewImageBuildResultSingleRef(id model.TargetID, ref reference.NamedTagged) ImageBuildResult {
	return NewImageBuildResult(id, ref, ref)
}

type DockerComposeBuildResult struct {
	id model.TargetID

	// When we deploy a Docker Compose service, we wait synchronously for the
	// container to start. Note that this is a different concurrency model than
	// we use for Kubernetes, where the pods appear some time later via an
	// asynchronous event.
	Status v1alpha1.DockerComposeServiceStatus
}

func (r DockerComposeBuildResult) TargetID() model.TargetID   { return r.id }
func (r DockerComposeBuildResult) BuildType() model.BuildType { return model.BuildTypeDockerCompose }

// For docker compose deploy targets.
func NewDockerComposeDeployResult(id model.TargetID, status v1alpha1.DockerComposeServiceStatus) DockerComposeBuildResult {
	return DockerComposeBuildResult{
		id:     id,
		Status: status,
	}
}

type K8sBuildResult struct {
	*k8sconv.KubernetesApplyFilter

	id model.TargetID
}

func (r K8sBuildResult) TargetID() model.TargetID   { return r.id }
func (r K8sBuildResult) BuildType() model.BuildType { return model.BuildTypeK8s }

// NewK8sDeployResult creates a deploy result for Kubernetes deploy targets.
func NewK8sDeployResult(id model.TargetID, filter *k8sconv.KubernetesApplyFilter) K8sBuildResult {
	return K8sBuildResult{
		id:                    id,
		KubernetesApplyFilter: filter,
	}
}

func LocalImageRefFromBuildResult(r BuildResult) string {
	if r, ok := r.(ImageBuildResult); ok {
		return r.ImageMapStatus.ImageFromLocal
	}
	return ""
}

func ClusterImageRefFromBuildResult(r BuildResult) string {
	if r, ok := r.(ImageBuildResult); ok {
		return r.ImageMapStatus.ImageFromCluster
	}
	return ""
}

type BuildResultSet map[model.TargetID]BuildResult

func (set BuildResultSet) ApplyFilter() *k8sconv.KubernetesApplyFilter {
	for _, r := range set {
		r, ok := r.(K8sBuildResult)
		if ok {
			return r.KubernetesApplyFilter
		}
	}
	return nil
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

	// The default cluster.
	Cluster *v1alpha1.Cluster
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

func (b BuildState) ClusterOrEmpty() *v1alpha1.Cluster {
	if b.Cluster == nil {
		return &v1alpha1.Cluster{}
	}
	return b.Cluster
}

func (b BuildState) WithFullBuildTriggered(isImageBuildTrigger bool) BuildState {
	b.FullBuildTriggered = isImageBuildTrigger
	return b
}

func (b BuildState) LastLocalImageAsString() string {
	return LocalImageRefFromBuildResult(b.LastResult)
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

var BuildStateClean = BuildState{}
