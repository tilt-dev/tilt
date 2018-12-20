package model

import (
	"reflect"
	"sort"

	"github.com/docker/distribution/reference"
)

type buildInfo interface {
	buildInfo()
}

type buildDetails interface {
	buildDetails()
}

type DockerInfo struct {
	CachePaths   []string
	DockerRef    reference.Named
	buildDetails buildDetails
}

func (DockerInfo) buildInfo()     {}
func (di DockerInfo) Empty() bool { return reflect.DeepEqual(di, DockerInfo{}) }

func (di DockerInfo) WithBuildDetails(details buildDetails) DockerInfo {
	di.buildDetails = details
	return di
}

func (di DockerInfo) WithCachePaths(paths []string) DockerInfo {
	di.CachePaths = append(append([]string{}, di.CachePaths...), paths...)
	sort.Strings(di.CachePaths)
	return di
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
