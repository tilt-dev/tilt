package model

import (
	"fmt"
	"path/filepath"
	"sort"

	"github.com/docker/distribution/reference"
	"github.com/windmilleng/tilt/internal/sliceutils"
)

type ImageTarget struct {
	Ref          reference.Named
	BuildDetails BuildDetails

	cachePaths []string

	// TODO(nick): It might eventually make sense to represent
	// Tiltfile as a separate nodes in the build graph, rather
	// than duplicating it in each ImageTarget.
	tiltFilename  string
	dockerignores []Dockerignore
	repos         []LocalGitRepo
	dependencyIDs []TargetID
}

func ImageID(ref reference.Named) TargetID {
	name := TargetName("")
	if ref != nil {
		name = TargetName(ref.Name())
	}
	return TargetID{
		Type: TargetTypeImage,
		Name: name,
	}
}

func (i ImageTarget) ID() TargetID {
	return ImageID(i.Ref)
}

func (i ImageTarget) DependencyIDs() []TargetID {
	return i.dependencyIDs
}

func (i ImageTarget) WithDependencyIDs(ids []TargetID) ImageTarget {
	i.dependencyIDs = DedupeTargetIDs(ids)
	return i
}

func (i ImageTarget) Validate() error {
	if i.Ref == nil {
		return fmt.Errorf("[Validate] Image target missing image ref: %+v", i.BuildDetails)
	}

	switch bd := i.BuildDetails.(type) {
	case StaticBuild:
		if bd.BuildPath == "" {
			return fmt.Errorf("[Validate] Image %q missing build path", i.Ref)
		}
	case FastBuild:
		if bd.BaseDockerfile == "" {
			return fmt.Errorf("[Validate] Image %q missing base dockerfile", i.Ref)
		}

		for _, mnt := range bd.Mounts {
			if !filepath.IsAbs(mnt.LocalPath) {
				return fmt.Errorf(
					"[Validate] Image %q: mount must be an absolute path (got: %s)", i.Ref, mnt.LocalPath)
			}
		}

	default:
		return fmt.Errorf("[Validate] Image %q has neither StaticBuildInfo nor FastBuildInfo", i.Ref)
	}

	return nil
}

type BuildDetails interface {
	buildDetails()
}

func (i ImageTarget) StaticBuildInfo() StaticBuild {
	ret, _ := i.BuildDetails.(StaticBuild)
	return ret
}

func (i ImageTarget) IsStaticBuild() bool {
	_, ok := i.BuildDetails.(StaticBuild)
	return ok
}

func (i ImageTarget) FastBuildInfo() FastBuild {
	ret, _ := i.BuildDetails.(FastBuild)
	return ret
}

func (i ImageTarget) IsFastBuild() bool {
	_, ok := i.BuildDetails.(FastBuild)
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

func (i ImageTarget) Dockerignores() []Dockerignore {
	return append([]Dockerignore{}, i.dockerignores...)
}

func (i ImageTarget) LocalPaths() []string {
	switch bd := i.BuildDetails.(type) {
	case StaticBuild:
		return []string{bd.BuildPath}
	case FastBuild:
		result := make([]string, len(bd.Mounts))
		for i, mount := range bd.Mounts {
			result[i] = mount.LocalPath
		}
		return result
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

type StaticBuild struct {
	Dockerfile string
	BuildPath  string // the absolute path to the files
	BuildArgs  DockerBuildArgs
}

func (StaticBuild) buildDetails() {}

type FastBuild struct {
	BaseDockerfile string
	Mounts         []Mount
	Steps          []Step
	Entrypoint     Cmd

	// A HotReload container image knows how to automatically
	// reload any changes in the container. No need to restart it.
	HotReload bool
}

func (FastBuild) buildDetails() {}

var _ TargetSpec = ImageTarget{}
