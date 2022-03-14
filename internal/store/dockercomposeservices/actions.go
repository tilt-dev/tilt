package dockercomposeservices

import "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

type DockerComposeServiceUpsertAction struct {
	DockerComposeService *v1alpha1.DockerComposeService
}

func NewDockerComposeServiceUpsertAction(obj *v1alpha1.DockerComposeService) DockerComposeServiceUpsertAction {
	return DockerComposeServiceUpsertAction{DockerComposeService: obj}
}

func (DockerComposeServiceUpsertAction) Action() {}

type DockerComposeServiceDeleteAction struct {
	Name string
}

func NewDockerComposeServiceDeleteAction(n string) DockerComposeServiceDeleteAction {
	return DockerComposeServiceDeleteAction{Name: n}
}

func (DockerComposeServiceDeleteAction) Action() {}
