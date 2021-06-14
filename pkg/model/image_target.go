package model

import (
	"fmt"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/sliceutils"
)

type ImageTarget struct {
	Refs           container.RefSet
	BuildDetails   BuildDetails
	MatchInEnvVars bool

	// User-supplied command to run when the container runs
	// (i.e. overrides k8s yaml "command", container ENTRYPOINT, etc.)
	OverrideCmd Cmd

	// User-supplied args override when the container runs.
	// (i.e. overrides k8s yaml "args")
	OverrideArgs OverrideArgs

	// TODO(nick): It might eventually make sense to represent
	// Tiltfile as a separate nodes in the build graph, rather
	// than duplicating it in each ImageTarget.
	tiltFilename  string
	dockerignores []Dockerignore
	repos         []LocalGitRepo
	dependencyIDs []TargetID
}

// Represent OverrideArgs as a special struct, to cleanly distinguish
// "replace with 0 args" from "don't replace"
type OverrideArgs struct {
	ShouldOverride bool
	Args           []string
}

func MustNewImageTarget(ref container.RefSelector) ImageTarget {
	return ImageTarget{Refs: container.MustSimpleRefSet(ref)}
}

func ImageID(ref container.RefSelector) TargetID {
	name := TargetName("")
	if !ref.Empty() {
		name = TargetName(container.FamiliarString(ref))
	}
	return TargetID{
		Type: TargetTypeImage,
		Name: name,
	}
}

func (i ImageTarget) ID() TargetID {
	return ImageID(i.Refs.ConfigurationRef)
}

func (i ImageTarget) DependencyIDs() []TargetID {
	return i.dependencyIDs
}

func (i ImageTarget) WithDependencyIDs(ids []TargetID) ImageTarget {
	i.dependencyIDs = DedupeTargetIDs(ids)
	return i
}

func (i ImageTarget) Validate() error {
	confRef := i.Refs.ConfigurationRef
	if confRef.Empty() {
		return fmt.Errorf("[Validate] Image target missing image ref: %+v", i.BuildDetails)
	}

	if err := i.Refs.Validate(); err != nil {
		return fmt.Errorf("[Validate] Image %q refset failed validation: %v", confRef, err)
	}

	switch bd := i.BuildDetails.(type) {
	case DockerBuild:
		if bd.BuildPath == "" {
			return fmt.Errorf("[Validate] Image %q missing build path", confRef)
		}
	case CustomBuild:
		if bd.Command.Empty() {
			return fmt.Errorf(
				"[Validate] CustomBuild command must not be empty",
			)
		}
	default:
		return fmt.Errorf("[Validate] Image %q has neither DockerBuild nor "+
			"CustomBuild details", confRef)
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

func (i ImageTarget) LiveUpdateInfo() LiveUpdate {
	switch details := i.BuildDetails.(type) {
	case DockerBuild:
		return details.LiveUpdate
	case CustomBuild:
		return details.LiveUpdate
	default:
		return LiveUpdate{}
	}
}

func (i ImageTarget) CustomBuildInfo() CustomBuild {
	ret, _ := i.BuildDetails.(CustomBuild)
	return ret
}

func (i ImageTarget) IsCustomBuild() bool {
	_, ok := i.BuildDetails.(CustomBuild)
	return ok
}

func (i ImageTarget) WithBuildDetails(details BuildDetails) ImageTarget {
	i.BuildDetails = details
	return i
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
func (i ImageTarget) MaybeIgnoreRegistry() ImageTarget {
	customBuild, ok := i.BuildDetails.(CustomBuild)
	if ok && customBuild.SkipsLocalDocker && customBuild.Tag != "" {
		i.Refs = i.Refs.WithoutRegistry()
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
	i.OverrideCmd = cmd
	return i
}

func (i ImageTarget) Dockerignores() []Dockerignore {
	return append([]Dockerignore{}, i.dockerignores...)
}

func (i ImageTarget) LocalPaths() []string {
	switch bd := i.BuildDetails.(type) {
	case DockerBuild:
		return []string{bd.BuildPath}
	case CustomBuild:
		return append([]string(nil), bd.Deps...)
	}
	return nil
}

func (i ImageTarget) LocalRepos() []LocalGitRepo {
	return i.repos
}

func (i ImageTarget) IgnoredLocalDirectories() []string {
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

func ImageTargetsByID(iTargets []ImageTarget) map[TargetID]ImageTarget {
	result := make(map[TargetID]ImageTarget, len(iTargets))
	for _, target := range iTargets {
		result[target.ID()] = target
	}
	return result
}

type DockerBuild struct {
	Dockerfile  string
	BuildPath   string // the absolute path to the files
	BuildArgs   DockerBuildArgs
	LiveUpdate  LiveUpdate // Optionally, can use LiveUpdate to update this build in place.
	TargetStage DockerBuildTarget

	// Pass SSH secrets to docker so it can clone private repos.
	// https://docs.docker.com/develop/develop-images/build_enhancements/#using-ssh-to-access-private-data-in-builds
	SSHSpecs []string

	// Pass secrets to docker
	// https://docs.docker.com/develop/develop-images/build_enhancements/#new-docker-build-secret-information
	SecretSpecs []string

	Network string

	PullParent bool
	CacheFrom  []string

	// By default, Tilt creates a new temporary image reference for each build.
	// The user can also specify their own reference, to integrate with other tooling
	// (like build IDs for Jenkins build pipelines)
	//
	// Equivalent to the docker build --tag flag.
	// Named 'tag' for consistency with how it's used throughout the docker API,
	// even though this is really more like a reference.NamedTagged
	ExtraTags []string
}

func (DockerBuild) buildDetails() {}

type DockerBuildTarget string

func (s DockerBuildTarget) String() string { return string(s) }

type CustomBuild struct {
	WorkDir string
	Command Cmd
	// Deps is a list of file paths that are dependencies of this command.
	Deps []string

	// Optional: tag we expect the image to be built with (we use this to check that
	// the expected image+tag has been created).
	// If empty, we create an expected tag at the beginning of CustomBuild (and
	// export $EXPECTED_REF=name:expected_tag )
	Tag string

	LiveUpdate       LiveUpdate // Optionally, can use LiveUpdate to update this build in place.
	DisablePush      bool
	SkipsLocalDocker bool

	// We expect the custom build script to print the image ref to this file,
	// so that Tilt can read it out when we're done.
	OutputsImageRefTo string
}

func (CustomBuild) buildDetails() {}

func (cb CustomBuild) WithTag(t string) CustomBuild {
	cb.Tag = t
	return cb
}

func (cb CustomBuild) SkipsPush() bool {
	return cb.SkipsLocalDocker || cb.DisablePush
}

var _ TargetSpec = ImageTarget{}
