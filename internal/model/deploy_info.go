package model

import (
	"sort"

	"github.com/windmilleng/tilt/internal/sliceutils"
	"github.com/windmilleng/tilt/internal/yaml"
)

type DockerComposeTarget struct {
	Name       TargetName
	ConfigPath string
	Mounts     []Mount
	YAMLRaw    []byte // for diff'ing when config files change
	DfRaw      []byte // for diff'ing when config files change

	// TODO(nick): It might eventually make sense to represent
	// Tiltfile as a separate nodes in the build graph, rather
	// than duplicating it in each DockerComposeTarget.
	tiltFilename  string
	dockerignores []Dockerignore
	repos         []LocalGitRepo
	// These directories and their children will not trigger file change events
	ignoredLocalDirectories []string
}

func (t DockerComposeTarget) ID() TargetID {
	return TargetID{
		Type: TargetTypeDockerCompose,
		Name: t.Name,
	}
}

func (t DockerComposeTarget) LocalPaths() []string {
	result := make([]string, len(t.Mounts))
	for i, mount := range t.Mounts {
		result[i] = mount.LocalPath
	}
	return result
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
	// TODO(dmiller) we can know the length of this slice
	deps := []string{}

	for _, p := range t.LocalPaths() {
		deps = append(deps, p)
	}

	deduped := sliceutils.DedupeStringSlice(deps)

	// Sort so that any nested paths come after their parents
	sort.Strings(deduped)

	return deduped
}

type K8sTarget struct {
	Name         TargetName
	YAML         string
	PortForwards []PortForward
}

func (k8s K8sTarget) ID() TargetID {
	return TargetID{
		Type: TargetTypeK8s,
		Name: k8s.Name,
	}
}

func (k8s K8sTarget) AppendYAML(y string) K8sTarget {
	if k8s.YAML == "" {
		k8s.YAML = y
	} else {
		k8s.YAML = yaml.ConcatYAML(k8s.YAML, y)
	}
	return k8s
}

var _ TargetSpec = K8sTarget{}
var _ TargetSpec = DockerComposeTarget{}
