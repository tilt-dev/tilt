package model

import (
	"reflect"
	"sort"

	"github.com/docker/distribution/reference"
)

type DockerInfo struct {
	cachePaths   []string
	DockerRef    reference.Named
	buildDetails buildDetails
}

type buildDetails interface {
	buildDetails()
}

func (di DockerInfo) WithBuildDetails(details buildDetails) DockerInfo {
	di.buildDetails = details
	return di
}

func (di DockerInfo) WithCachePaths(paths []string) DockerInfo {
	di.cachePaths = append(append([]string{}, di.cachePaths...), paths...)
	sort.Strings(di.cachePaths)
	return di
}

func (di DockerInfo) CachePaths() []string {
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
