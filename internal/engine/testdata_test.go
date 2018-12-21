package engine

import (
	"github.com/docker/distribution/reference"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/model"
)

const SanchoYAML = testyaml.SanchoYAML

const SanchoTwinYAML = testyaml.SanchoTwinYAML

const SanchoBaseDockerfile = `
FROM go:1.10
`

const SanchoStaticDockerfile = `
FROM go:1.10
ADD . .
RUN go install github.com/windmilleng/sancho
ENTRYPOINT /go/bin/sancho
`

var SanchoRef, _ = reference.ParseNormalizedNamed("gcr.io/some-project-162817/sancho")

func NewSanchoManifest() model.Manifest {
	m := model.Manifest{
		Name: "sancho",
		DockerInfo: model.DockerInfo{
			DockerRef: SanchoRef,
		},
		BaseDockerfile: SanchoBaseDockerfile,
		Mounts: []model.Mount{
			model.Mount{
				LocalPath:     "/src/sancho",
				ContainerPath: "/go/src/github.com/windmilleng/sancho",
			},
		},
		Steps: model.ToSteps("/", []model.Cmd{
			model.Cmd{Argv: []string{"go", "install", "github.com/windmilleng/sancho"}},
		}),
		Entrypoint: model.Cmd{Argv: []string{"/go/bin/sancho"}},
	}

	m = m.WithDeployInfo(model.K8sInfo{YAML: SanchoYAML})

	return m
}

func NewSanchoManifestWithCache(paths []string) model.Manifest {
	manifest := NewSanchoManifest()
	manifest.DockerInfo = manifest.DockerInfo.WithCachePaths(paths)
	return manifest
}

func NewSanchoStaticManifest() model.Manifest {
	m := model.Manifest{
		Name: "sancho",
		DockerInfo: model.DockerInfo{
			DockerRef: SanchoRef,
		}.WithBuildDetails(model.StaticBuild{
			Dockerfile: SanchoStaticDockerfile,
			BuildPath:  "/path/to/build",
		}),
	}.WithDeployInfo(model.K8sInfo{YAML: SanchoYAML})
	return m
}

func NewSanchoStaticManifestWithCache(paths []string) model.Manifest {
	manifest := NewSanchoStaticManifest()
	manifest.DockerInfo = manifest.DockerInfo.WithCachePaths(paths)
	return manifest
}

var SanchoManifest = NewSanchoManifest()
var SanchoStaticManifest = NewSanchoStaticManifest()
