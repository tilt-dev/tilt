package tiltfile

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/pkg/model"
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
	f.file(filepath.Join("src", "index.html"), "Hello world!")
	f.file(filepath.Join("src", ".dockerignore"), "**")

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

func TestCustomBuldDepsAreLocalRepos(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.yaml("fe.yaml", deployment("fe", image("gcr.io/fe")))
	f.file("Dockerfile", `
FROM alpine
ADD . .
`)
	f.file("Tiltfile", `
k8s_yaml('fe.yaml')
custom_build('gcr.io/fe', 'docker build -t $EXPECTED_REF .', ['src'])
`)
	f.file(".dockerignore", "build")
	f.file(filepath.Join("src", "index.html"), "Hello world!")
	f.file(filepath.Join("src", ".git", "hi"), "hi")

	f.load()

	m := f.assertNextManifest("fe")
	it := m.ImageTargets[0]

	var localPathStrings []string
	for _, r := range it.LocalRepos() {
		localPathStrings = append(localPathStrings, r.LocalPath)
	}

	assert.Contains(t, localPathStrings, f.JoinPath("src"))
}
