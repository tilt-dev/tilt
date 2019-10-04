package tiltfile

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/pkg/model"
)

func TestDockerignoreInSyncDir(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.yaml("fe.yaml", deployment("fe", image("gcr.io/fe")))
	f.file("Dockerfile", `
FROM alpine
ADD ./src /src
`)
	f.file("Tiltfile", `
k8s_yaml('fe.yaml')
docker_build('gcr.io/fe', '.', live_update=[
  sync('./src', '/src')
])
`)
	f.file(".dockerignore", "build")
	f.file("src/index.html", "Hello world!")
	f.file("src/.dockerignore", "**")

	f.load()
	m := f.assertNextManifest("fe")
	assert.Equal(t,
		[]model.Dockerignore{
			model.Dockerignore{
				LocalPath: f.Path(),
				Contents:  "build",
			},
		},
		m.ImageTargetAt(0).Dockerignores())
}
