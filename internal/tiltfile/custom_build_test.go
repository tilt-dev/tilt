package tiltfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCustomBuildImageDeps(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("Tiltfile", `
custom_build(
  'base',
  'build.sh',
  ['.']
)
custom_build(
  'fe',
  'build.sh',
  ['.'],
  image_deps=['base'],
)
k8s_yaml('fe.yaml')
`)
	f.file("Dockerfile", "FROM alpine")
	f.yaml("fe.yaml", deployment("fe", image("fe")))

	f.load()

	m := f.assertNextManifest("fe")
	if assert.Equal(t, 2, len(m.ImageTargets)) {
		assert.Equal(t, []string{"base"}, m.ImageTargets[1].CustomBuildInfo().ImageMaps)
	}
}

func TestCustomBuildMissingImageDeps(t *testing.T) {
	f := newFixture(t)
	defer f.TearDown()

	f.file("Tiltfile", `
custom_build(
  'fe',
  'build.sh',
  ['.'],
  image_deps=['base'],
)
k8s_yaml('fe.yaml')
`)
	f.file("Dockerfile", "FROM alpine")
	f.yaml("fe.yaml", deployment("fe", image("fe")))

	f.loadErrString(`image "fe": image dep "base" not found`)
}
