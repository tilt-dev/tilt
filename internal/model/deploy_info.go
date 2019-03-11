package model

import (
	"fmt"
	"reflect"
	"strconv"
	"time"

	"k8s.io/apimachinery/pkg/labels"

	"github.com/windmilleng/tilt/internal/sliceutils"
	"github.com/windmilleng/tilt/internal/yaml"
)

type DeployID int64 // Unix ns after epoch -- uniquely identify a deploy

func NewDeployID() DeployID {
	return DeployID(time.Now().UnixNano())
}

func (dID DeployID) String() string { return strconv.Itoa(int(dID)) }

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

	dependencyIDs []TargetID
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
	result := make([]string, len(t.Mounts))
	for i, mount := range t.Mounts {
		result[i] = mount.LocalPath
	}
	return result
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
		return fmt.Errorf("[Validate] DockerCompose resource missing name:\n%s", dc.YAMLRaw)
	}

	if dc.ConfigPath == "" {
		return fmt.Errorf("[Validate] DockerCompose resource %s missing config path", dc.Name)
	}

	return nil
}

type K8sTarget struct {
	Name         TargetName
	YAML         string
	PortForwards []PortForward
	// labels for pods that we should watch and associate with this resource
	ExtraPodSelectors []labels.Selector
	ResourceNames     []string

	dependencyIDs []TargetID
}

func (k8s K8sTarget) Empty() bool { return reflect.DeepEqual(k8s, K8sTarget{}) }

func (k8s K8sTarget) DependencyIDs() []TargetID {
	return k8s.dependencyIDs
}

func (k8s K8sTarget) Validate() error {
	if k8s.ID().Empty() {
		return fmt.Errorf("[Validate] K8s resources missing name:\n%s", k8s.YAML)
	}

	if k8s.YAML == "" {
		return fmt.Errorf("[Validate] K8s resources %q missing YAML", k8s.Name)
	}

	return nil
}

func (k8s K8sTarget) ID() TargetID {
	return TargetID{
		Type: TargetTypeK8s,
		Name: k8s.Name,
	}
}

func (k8s K8sTarget) WithDependencyIDs(ids []TargetID) K8sTarget {
	k8s.dependencyIDs = DedupeTargetIDs(ids)
	return k8s
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
