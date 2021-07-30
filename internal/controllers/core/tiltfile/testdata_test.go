package tiltfile

import (
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/pkg/model"
)

const SanchoDockerfile = `
FROM go:1.10
ADD . .
RUN go install github.com/tilt-dev/sancho
ENTRYPOINT /go/bin/sancho
`

var SanchoRef = container.MustParseSelector(testyaml.SanchoImage)

type pathFixture interface {
	Path() string
}

func NewSanchoDockerBuildImageTarget(f pathFixture) model.ImageTarget {
	return model.MustNewImageTarget(SanchoRef).WithBuildDetails(model.DockerBuild{
		Dockerfile: SanchoDockerfile,
		BuildPath:  f.Path(),
	})
}
