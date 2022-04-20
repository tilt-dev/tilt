package dockerimages

import "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

type DockerImageUpsertAction struct {
	DockerImage *v1alpha1.DockerImage
}

func NewDockerImageUpsertAction(obj *v1alpha1.DockerImage) DockerImageUpsertAction {
	return DockerImageUpsertAction{DockerImage: obj}
}

func (DockerImageUpsertAction) Action() {}

type DockerImageDeleteAction struct {
	Name string
}

func NewDockerImageDeleteAction(n string) DockerImageDeleteAction {
	return DockerImageDeleteAction{Name: n}
}

func (DockerImageDeleteAction) Action() {}
