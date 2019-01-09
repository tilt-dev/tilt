package model

import (
	"reflect"
	"sort"

	"github.com/docker/distribution/reference"
)

type ImageTarget struct {
	cachePaths   []string
	Ref          reference.Named
	BuildDetails BuildDetails
}

func (di ImageTarget) ID() TargetID {
	return TargetID{
		Type: TargetTypeImage,
		Name: di.Ref.String(),
	}
}

type BuildDetails interface {
	buildDetails()
}

func (di ImageTarget) WithBuildDetails(details BuildDetails) ImageTarget {
	di.BuildDetails = details
	return di
}

func (di ImageTarget) WithCachePaths(paths []string) ImageTarget {
	di.cachePaths = append(append([]string{}, di.cachePaths...), paths...)
	sort.Strings(di.cachePaths)
	return di
}

func (di ImageTarget) CachePaths() []string {
	return di.cachePaths
}

type StaticBuild struct {
	Dockerfile string
	BuildPath  string // the absolute path to the files
	BuildArgs  DockerBuildArgs
}

func (StaticBuild) buildDetails()  {}
func (sb StaticBuild) Empty() bool { return reflect.DeepEqual(sb, StaticBuild{}) }

type FastBuild struct {
	BaseDockerfile string
	Mounts         []Mount
	Steps          []Step
	Entrypoint     Cmd
}

func (FastBuild) buildDetails()  {}
func (fb FastBuild) Empty() bool { return reflect.DeepEqual(fb, FastBuild{}) }

var _ Target = ImageTarget{}
