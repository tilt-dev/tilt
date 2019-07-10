package model

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sort"

	"github.com/docker/distribution/reference"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/sliceutils"
)

type ImageTarget struct {
	ConfigurationRef container.RefSelector
	DeploymentRef    reference.Named
	BuildDetails     BuildDetails
	MatchInEnvVars   bool

	// User-supplied command to run when the container runs
	// (i.e. overrides k8s yaml "command", container ENTRYPOINT, etc.)
	OverrideCmd Cmd

	cachePaths []string

	// TODO(nick): It might eventually make sense to represent
	// Tiltfile as a separate nodes in the build graph, rather
	// than duplicating it in each ImageTarget.
	tiltFilename  string
	dockerignores []Dockerignore
	repos         []LocalGitRepo
	dependencyIDs []TargetID
}

func NewImageTarget(ref container.RefSelector) ImageTarget {
	return ImageTarget{ConfigurationRef: ref, DeploymentRef: ref.AsNamedOnly()}
}

func ImageID(ref container.RefSelector) TargetID {
	name := TargetName("")
	if !ref.Empty() {
		name = TargetName(ref.String())
	}
	return TargetID{
		Type: TargetTypeImage,
		Name: name,
	}
}

func (i ImageTarget) ID() TargetID {
	return ImageID(i.ConfigurationRef)
}

func (i ImageTarget) DependencyIDs() []TargetID {
	return i.dependencyIDs
}

func (i ImageTarget) WithDependencyIDs(ids []TargetID) ImageTarget {
	i.dependencyIDs = DedupeTargetIDs(ids)
	return i
}

func (i ImageTarget) Validate() error {
	if i.ConfigurationRef.Empty() {
		return fmt.Errorf("[Validate] Image target missing image ref: %+v", i.BuildDetails)
	}

	switch bd := i.BuildDetails.(type) {
	case DockerBuild:
		if bd.BuildPath == "" {
			return fmt.Errorf("[Validate] Image %q missing build path", i.ConfigurationRef)
		}
	case FastBuild:
		if bd.BaseDockerfile == "" {
			return fmt.Errorf("[Validate] Image %q missing base dockerfile", i.ConfigurationRef)
		}

		for _, mnt := range bd.Syncs {
			if !filepath.IsAbs(mnt.LocalPath) {
				return fmt.Errorf(
					"[Validate] Image %q: mount must be an absolute path (got: %s)", i.ConfigurationRef, mnt.LocalPath)
			}
		}
	case CustomBuild:
		if bd.Command == "" {
			return fmt.Errorf(
				"[Validate] CustomBuild command must not be empty",
			)
		}
	default:
		return fmt.Errorf("[Validate] Image %q has neither DockerBuildInfo nor FastBuildInfo", i.ConfigurationRef)
	}

	return nil
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

func (i ImageTarget) AnyLiveUpdateInfo() LiveUpdate {
	switch details := i.BuildDetails.(type) {
	case DockerBuild:
		return details.LiveUpdate
	case CustomBuild:
		return details.LiveUpdate
	default:
		return LiveUpdate{}
	}
}

func (i ImageTarget) AnyFastBuildInfo() FastBuild {
	switch details := i.BuildDetails.(type) {
	case FastBuild:
		return details
	case DockerBuild:
		return details.FastBuild
	case CustomBuild:
		return details.Fast
	}
	return FastBuild{}
}

// FastBuildInfo returns the TOP LEVEL BUILD DETAILS, if a FastBuild.
// Does not return nested FastBuild details.
func (i ImageTarget) TopFastBuildInfo() FastBuild {
	ret, _ := i.BuildDetails.(FastBuild)
	return ret
}

// IsFastBuild checks if the TOP LEVEL BUILD DETAILS are for a FastBuild.
// (If this target is a DockerBuild || CustomBuild with a nested FastBuild, returns FALSE.)
func (i ImageTarget) IsFastBuild() bool {
	_, ok := i.BuildDetails.(FastBuild)
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

func (i ImageTarget) WithBuildDetails(details BuildDetails) ImageTarget {
	i.BuildDetails = details
	return i
}

func (i ImageTarget) WithCachePaths(paths []string) ImageTarget {
	i.cachePaths = append(append([]string{}, i.cachePaths...), paths...)
	sort.Strings(i.cachePaths)
	return i
}

func (i ImageTarget) CachePaths() []string {
	return i.cachePaths
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
	case FastBuild:
		result := make([]string, len(bd.Syncs))
		for i, mount := range bd.Syncs {
			result[i] = mount.LocalPath
		}
		return result
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
	Dockerfile string
	BuildPath  string // the absolute path to the files
	BuildArgs  DockerBuildArgs
	FastBuild  FastBuild  // Optionally, can use FastBuild to update this build in place.
	LiveUpdate LiveUpdate // Optionally, can use LiveUpdate to update this build in place.
}

func (DockerBuild) buildDetails() {}

type FastBuild struct {
	BaseDockerfile string
	Syncs          []Sync
	Runs           []Run
	Entrypoint     Cmd

	// A HotReload container image knows how to automatically
	// reload any changes in the container. No need to restart it.
	HotReload bool
}

func (FastBuild) buildDetails()  {}
func (fb FastBuild) Empty() bool { return reflect.DeepEqual(fb, FastBuild{}) }

type CustomBuild struct {
	Command string
	// Deps is a list of file paths that are dependencies of this command.
	Deps []string

	// Optional: tag we expect the image to be built with (we use this to check that
	// the expected image+tag has been created).
	// If empty, we create an expected tag at the beginning of CustomBuild (and
	// export $EXPECTED_REF=name:expected_tag )
	Tag string

	Fast        FastBuild
	LiveUpdate  LiveUpdate // Optionally, can use LiveUpdate to update this build in place.
	DisablePush bool
}

func (CustomBuild) buildDetails() {}

func (cb CustomBuild) WithTag(t string) CustomBuild {
	cb.Tag = t
	return cb
}

var _ TargetSpec = ImageTarget{}
