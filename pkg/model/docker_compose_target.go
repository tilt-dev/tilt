package model

import (
	"fmt"

	"github.com/tilt-dev/tilt/internal/sliceutils"
)

type DockerComposeTarget struct {
	Spec DockerComposeUpSpec

	Name TargetName

	// The docker context, like in DockerBuild
	buildPath string

	ServiceYAML string // for diff'ing when config files change

	DfRaw []byte // for diff'ing when config files change

	// TODO(nick): It might eventually make sense to represent
	// Tiltfile as a separate nodes in the build graph, rather
	// than duplicating it in each DockerComposeTarget.
	tiltFilename  string
	dockerignores []Dockerignore
	repos         []LocalGitRepo
	// These directories and their children will not trigger file change events
	ignoredLocalDirectories []string

	dependencyIDs []TargetID

	publishedPorts []int

	Links []Link
}

// TODO(nick): This is a temporary hack until we figure out how we want
// to pass these IDs to the docker-compose UX.
func (t DockerComposeTarget) ManifestName() ManifestName {
	return ManifestName(t.Name)
}

func (t DockerComposeTarget) Empty() bool { return t.ID().Empty() }

func (t DockerComposeTarget) ID() TargetID {
	return TargetID{
		Type: TargetTypeDockerCompose,
		Name: t.Name,
	}
}

func (t DockerComposeTarget) DependencyIDs() []TargetID {
	return t.dependencyIDs
}

func (t DockerComposeTarget) LocalPaths() []string {
	if t.buildPath == "" {
		return []string{}
	}
	return []string{t.buildPath}
}

func (t DockerComposeTarget) PublishedPorts() []int {
	return append([]int{}, t.publishedPorts...)
}

func (t DockerComposeTarget) WithLinks(links []Link) DockerComposeTarget {
	t.Links = links
	return t
}

func (t DockerComposeTarget) WithPublishedPorts(ports []int) DockerComposeTarget {
	t.publishedPorts = ports
	return t
}

func (t DockerComposeTarget) WithBuildPath(buildPath string) DockerComposeTarget {
	t.buildPath = buildPath
	return t
}

func (t DockerComposeTarget) WithDependencyIDs(ids []TargetID) DockerComposeTarget {
	t.dependencyIDs = DedupeTargetIDs(ids)
	return t
}

func (t DockerComposeTarget) WithRepos(repos []LocalGitRepo) DockerComposeTarget {
	t.repos = append(append([]LocalGitRepo{}, t.repos...), repos...)
	return t
}

func (t DockerComposeTarget) WithDockerignores(dockerignores []Dockerignore) DockerComposeTarget {
	t.dockerignores = append(append([]Dockerignore{}, t.dockerignores...), dockerignores...)
	return t
}

func (t DockerComposeTarget) Dockerignores() []Dockerignore {
	return append([]Dockerignore{}, t.dockerignores...)
}

func (t DockerComposeTarget) LocalRepos() []LocalGitRepo {
	return t.repos
}

func (t DockerComposeTarget) IgnoredLocalDirectories() []string {
	return t.ignoredLocalDirectories
}

func (t DockerComposeTarget) TiltFilename() string {
	return t.tiltFilename
}

func (t DockerComposeTarget) WithTiltFilename(f string) DockerComposeTarget {
	t.tiltFilename = f
	return t
}

func (t DockerComposeTarget) WithIgnoredLocalDirectories(dirs []string) DockerComposeTarget {
	t.ignoredLocalDirectories = dirs
	return t
}

// TODO(nick): This method should be deleted. We should just de-dupe and sort LocalPaths once
// when we create it, rather than have a duplicate method that does the "right" thing.
func (t DockerComposeTarget) Dependencies() []string {
	return sliceutils.DedupedAndSorted(t.LocalPaths())
}

func (dc DockerComposeTarget) Validate() error {
	if dc.ID().Empty() {
		return fmt.Errorf("[Validate] DockerCompose resource missing name:\n%s", dc.ServiceYAML)
	}

	if len(dc.Spec.Project.ConfigPaths) == 0 && dc.Spec.Project.YAML == "" {
		return fmt.Errorf("[Validate] DockerCompose resource %s missing config path", dc.Spec.Service)
	}

	return nil
}

var _ TargetSpec = DockerComposeTarget{}
