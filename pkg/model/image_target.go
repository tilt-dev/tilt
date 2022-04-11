package model

import (
	"fmt"

	"github.com/google/go-cmp/cmp"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type ImageTarget struct {
	// An apiserver-driven data model for injecting the image into other resources.
	v1alpha1.ImageMapSpec

	// An apiserver-driven data model for live-updating containers.
	LiveUpdateName       string
	LiveUpdateSpec       v1alpha1.LiveUpdateSpec
	LiveUpdateReconciler bool

	// An apiserver-driven data model for using docker to build images.
	DockerImageName string
	CmdImageName    string

	Refs         container.RefSet
	BuildDetails BuildDetails

	// In a live-update-only image, we don't inject the image into the Kubernetes
	// deploy, we only live-update to the deployed object. See this issue:
	//
	// https://github.com/tilt-dev/tilt/issues/4577
	//
	// This is a hacky way to model this right now until we
	// firm up how images work in the apiserver.
	IsLiveUpdateOnly bool

	// TODO(nick): It might eventually make sense to represent
	// Tiltfile as a separate nodes in the build graph, rather
	// than duplicating it in each ImageTarget.
	tiltFilename  string
	dockerignores []Dockerignore
	repos         []LocalGitRepo
}

var _ TargetSpec = ImageTarget{}

func MustNewImageTarget(ref container.RefSelector) ImageTarget {
	return ImageTarget{}.MustWithRef(ref)
}

func ImageID(ref container.RefSelector) TargetID {
	name := TargetName("")
	if !ref.Empty() {
		name = TargetName(apis.SanitizeName(container.FamiliarString(ref)))
	}
	return TargetID{
		Type: TargetTypeImage,
		Name: name,
	}
}

func (i ImageTarget) ImageMapName() string {
	return i.ID().Name.String()
}

func (i ImageTarget) MustWithRef(ref container.RefSelector) ImageTarget {
	i.Refs = container.MustSimpleRefSet(ref)
	i.ImageMapSpec.Selector = ref.RefFamiliarString()
	i.ImageMapSpec.MatchExact = ref.MatchExact()
	return i
}

func (i ImageTarget) WithLiveUpdateSpec(name string, luSpec v1alpha1.LiveUpdateSpec) ImageTarget {
	if luSpec.Selector.Kubernetes == nil {
		luSpec.Selector.Kubernetes = i.LiveUpdateSpec.Selector.Kubernetes
	}
	i.LiveUpdateName = name
	i.LiveUpdateSpec = luSpec
	return i
}

func (i ImageTarget) ID() TargetID {
	return TargetID{
		Type: TargetTypeImage,
		Name: TargetName(apis.SanitizeName(i.ImageMapSpec.Selector)),
	}
}

func (i ImageTarget) DependencyIDs() []TargetID {
	deps := i.ImageMapDeps()
	result := make([]TargetID, 0, len(deps))
	for _, im := range deps {
		result = append(result, TargetID{
			Type: TargetTypeImage,
			Name: TargetName(im),
		})
	}
	return result
}

func (i ImageTarget) ImageMapDeps() []string {
	switch bd := i.BuildDetails.(type) {
	case DockerBuild:
		return bd.ImageMaps
	case CustomBuild:
		return bd.ImageMaps
	}
	return nil
}

func (i ImageTarget) WithImageMapDeps(names []string) ImageTarget {
	switch bd := i.BuildDetails.(type) {
	case DockerBuild:
		bd.ImageMaps = sliceutils.Dedupe(names)
		i.BuildDetails = bd
	case CustomBuild:
		bd.ImageMaps = sliceutils.Dedupe(names)
		i.BuildDetails = bd
	default:
		if len(names) > 0 {
			panic(fmt.Sprintf("image does not support image deps: %v", i.ID()))
		}
	}
	return i
}

func (i ImageTarget) Validate() error {
	if i.ImageMapSpec.Selector == "" {
		return fmt.Errorf("[Validate] Image target missing image ref: %+v", i.BuildDetails)
	}

	selector, err := container.SelectorFromImageMap(i.ImageMapSpec)
	if err != nil {
		return fmt.Errorf("[Validate]: %v", err)
	}

	refs, err := container.NewRefSet(selector, container.Registry{})
	if err != nil {
		return fmt.Errorf("[Validate]: %v", err)
	}

	if err := refs.Validate(); err != nil {
		return fmt.Errorf("[Validate] Image %q refset failed validation: %v", i.ImageMapSpec.Selector, err)
	}

	switch bd := i.BuildDetails.(type) {
	case DockerBuild:
		if bd.Context == "" {
			return fmt.Errorf("[Validate] Image %q missing build path", i.ImageMapSpec.Selector)
		}
	case CustomBuild:
		if !i.IsLiveUpdateOnly && len(bd.Args) == 0 {
			return fmt.Errorf(
				"[Validate] CustomBuild command must not be empty",
			)
		}
	case DockerComposeBuild:
		if bd.Service == "" {
			return fmt.Errorf("[Validate] DockerComposeBuild missing service name")
		}
	default:
		return fmt.Errorf(
			"[Validate] Image %q has unsupported %T build details", i.ImageMapSpec.Selector, bd)
	}

	return nil
}

// HasDistinctClusterRef indicates whether the image target has a ClusterRef
// distinct from LocalRef, i.e. if the image is addressed different from
// inside and outside the cluster.
func (i ImageTarget) HasDistinctClusterRef() bool {
	return i.Refs.LocalRef().String() != i.Refs.ClusterRef().String()
}

type BuildDetails interface {
	buildDetails()
}

func (i ImageTarget) DockerBuildInfo() DockerBuild {
	ret, _ := i.BuildDetails.(DockerBuild)
	return ret
}

func (i ImageTarget) IsDockerBuild() bool {
	_, ok := i.BuildDetails.(DockerBuild)
	return ok
}

func (i ImageTarget) CustomBuildInfo() CustomBuild {
	ret, _ := i.BuildDetails.(CustomBuild)
	return ret
}

func (i ImageTarget) IsCustomBuild() bool {
	_, ok := i.BuildDetails.(CustomBuild)
	return ok
}

func (i ImageTarget) DockerComposeBuildInfo() DockerComposeBuild {
	ret, _ := i.BuildDetails.(DockerComposeBuild)
	return ret
}

func (i ImageTarget) IsDockerComposeBuild() bool {
	_, ok := i.BuildDetails.(DockerComposeBuild)
	return ok
}

func (i ImageTarget) WithDockerImage(spec v1alpha1.DockerImageSpec) ImageTarget {
	return i.WithBuildDetails(DockerBuild{DockerImageSpec: spec})
}

func (i ImageTarget) WithBuildDetails(details BuildDetails) ImageTarget {
	i.BuildDetails = details

	cb, ok := details.(CustomBuild)
	isEmptyLiveUpdateSpec := len(i.LiveUpdateSpec.Syncs) == 0 && len(i.LiveUpdateSpec.Execs) == 0
	if ok && cmp.Equal(cb.Args, ToHostCmd(":").Argv) && !isEmptyLiveUpdateSpec {
		// NOTE(nick): This is a hack for the file_sync_only extension
		// until we come up with a real API for specifying live update
		// without an image build.
		i.IsLiveUpdateOnly = true
	}
	return i
}

func (i ImageTarget) WithRepos(repos []LocalGitRepo) ImageTarget {
	i.repos = append(append([]LocalGitRepo{}, i.repos...), repos...)
	return i
}

func (i ImageTarget) WithDockerignores(dockerignores []Dockerignore) ImageTarget {
	i.dockerignores = append(append([]Dockerignore{}, i.dockerignores...), dockerignores...)
	return i
}

func (i ImageTarget) WithOverrideCommand(cmd Cmd) ImageTarget {
	i.ImageMapSpec.OverrideCommand = &v1alpha1.ImageMapOverrideCommand{
		Command: cmd.Argv,
	}
	return i
}

func (i ImageTarget) Dockerignores() []Dockerignore {
	return append([]Dockerignore{}, i.dockerignores...)
}

func (i ImageTarget) LocalPaths() []string {
	switch bd := i.BuildDetails.(type) {
	case DockerBuild:
		return []string{bd.Context}
	case CustomBuild:
		return append([]string(nil), bd.Deps...)
	case DockerComposeBuild:
		return []string{bd.Context}
	}
	return nil
}

func (i ImageTarget) ClusterNeeds() v1alpha1.ClusterImageNeeds {
	switch bd := i.BuildDetails.(type) {
	case DockerBuild:
		return bd.DockerImageSpec.ClusterNeeds
	case CustomBuild:
		return bd.CmdImageSpec.ClusterNeeds
	}
	return v1alpha1.ClusterImageNeedsBase
}

func (i ImageTarget) LocalRepos() []LocalGitRepo {
	return i.repos
}

func (i ImageTarget) IgnoredLocalDirectories() []string {
	if bd, ok := i.BuildDetails.(DockerComposeBuild); ok {
		return bd.LocalVolumePaths
	}
	return nil
}

func (i ImageTarget) TiltFilename() string {
	return i.tiltFilename
}

func (i ImageTarget) WithTiltFilename(f string) ImageTarget {
	i.tiltFilename = f
	return i
}

// TODO(nick): This method should be deleted. We should just de-dupe and sort LocalPaths once
// when we create it, rather than have a duplicate method that does the "right" thing.
func (i ImageTarget) Dependencies() []string {
	return sliceutils.DedupedAndSorted(i.LocalPaths())
}

// Once images are in the API server, they'll depend on the cluster:
//
// - The RefSet (where the image is built) will depend on the cluster registry.
// - The architecture (the image chipset) will depend on the default arch of the cluster.
//
// In the meantime, we handle this by inferring them after tiltfile assembly.
func (i ImageTarget) InferImagePropertiesFromCluster(reg container.Registry, clusterNeeds v1alpha1.ClusterImageNeeds, clusterName string) (ImageTarget, error) {
	selector, err := container.SelectorFromImageMap(i.ImageMapSpec)
	if err != nil {
		return i, fmt.Errorf("validating image: %v", err)
	}

	refs, err := container.NewRefSet(selector, reg)
	if err != nil {
		return i, fmt.Errorf("applying image %s to registry %s: %v", i.ImageMapSpec.Selector, reg, err)
	}

	db, ok := i.BuildDetails.(DockerBuild)
	if ok {
		db.DockerImageSpec.Ref = i.ImageMapSpec.Selector
		db.DockerImageSpec.ClusterNeeds = clusterNeeds
		db.DockerImageSpec.Cluster = clusterName
		i.BuildDetails = db
	}

	cb, ok := i.BuildDetails.(CustomBuild)
	if ok {
		cb.CmdImageSpec.Ref = i.ImageMapSpec.Selector
		cb.CmdImageSpec.ClusterNeeds = clusterNeeds
		cb.CmdImageSpec.Cluster = clusterName
		i.BuildDetails = cb
	}

	// I (Nick) am deeply unhappy with the parameters of CustomBuild.  They're not
	// well-specified, and often interact in weird and unpredictable ways.  This
	// function is a good example.
	//
	// custom_build(tag) means "My custom_build script already has a tag that it
	// wants to use". In practice, it becomes the "You can't tell me what to do"
	// flag.
	//
	// custom_build(skips_local_docker) means "My custom_build script doesn't use
	// Docker for storage, so you shouldn't expect to find the image there." In
	// practice, it becomes the "You can't touch my outputs" flag.
	//
	// When used together, you have a script that takes no inputs and doesn't let Tilt
	// fix the outputs. So people use custom_build(tag=x, skips_local_docker=True) to
	// enable all sorts of off-road experimental image-building flows that need better
	// primitives.
	//
	// For now, when we detect this case, we strip off registry information, since
	// the script isn't going to use it anyway.  This is tightly coupled with
	// CustomBuilder, which already has similar logic for handling these two cases
	// together.
	customBuild, ok := i.BuildDetails.(CustomBuild)
	if ok && customBuild.OutputMode == v1alpha1.CmdImageOutputRemote && customBuild.OutputTag != "" {
		refs = refs.WithoutRegistry()
	}
	_, ok = i.BuildDetails.(DockerComposeBuild)
	if ok {
		refs = refs.WithoutRegistry()
	}

	i.Refs = refs

	return i, nil
}

func ImageTargetsByID(iTargets []ImageTarget) map[TargetID]ImageTarget {
	result := make(map[TargetID]ImageTarget, len(iTargets))
	for _, target := range iTargets {
		result[target.ID()] = target
	}
	return result
}

type DockerBuild struct {
	v1alpha1.DockerImageSpec
}

func (DockerBuild) buildDetails() {}

type CustomBuild struct {
	v1alpha1.CmdImageSpec

	// Deps is a list of file paths that are dependencies of this command.
	//
	// TODO(nick): This creates a FileWatch. We should add a RestartOn field
	// to CmdImageSpec that points to the FileWatch.
	Deps []string
}

func (CustomBuild) buildDetails() {}

func (cb CustomBuild) WithTag(t string) CustomBuild {
	cb.CmdImageSpec.OutputTag = t
	return cb
}

func (cb CustomBuild) SkipsPush() bool {
	return cb.OutputMode == v1alpha1.CmdImageOutputLocalDockerAndRemote ||
		cb.OutputMode == v1alpha1.CmdImageOutputRemote
}

type DockerComposeBuild struct {
	// Service is the name of the Docker Compose service as defined in docker-compose.yaml.
	Service string

	// Context is the build context absolute path.
	Context string

	// LocalVolumePaths are ignored for triggering builds but are still included in the build context.
	LocalVolumePaths []string
}

func (d DockerComposeBuild) buildDetails() {
}
