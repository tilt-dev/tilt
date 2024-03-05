package model

import (
	"fmt"

	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type DockerComposeTarget struct {
	Spec v1alpha1.DockerComposeServiceSpec

	Name TargetName

	ServiceYAML string // for diff'ing when config files change

	publishedPorts []int

	InferLinks bool
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
	result := make([]TargetID, 0, len(t.Spec.ImageMaps))
	for _, im := range t.Spec.ImageMaps {
		result = append(result, TargetID{
			Type: TargetTypeImage,
			Name: TargetName(im),
		})
	}
	return result
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

func (t DockerComposeTarget) WithImageMapDeps(names []string) DockerComposeTarget {
	t.Spec.ImageMaps = sliceutils.Dedupe(names)
	return t
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
