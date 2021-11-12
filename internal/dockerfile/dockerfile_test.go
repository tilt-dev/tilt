package dockerfile

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindImages(t *testing.T) {
	df := Dockerfile(`FROM gcr.io/image-a`)
	images, err := df.FindImages(nil)
	assert.NoError(t, err)
	if assert.Equal(t, 1, len(images)) {
		assert.Equal(t, "gcr.io/image-a", images[0].String())
	}
}

func TestFindImagesAsBuilder(t *testing.T) {
	df := Dockerfile(`FROM gcr.io/image-a as builder`)
	images, err := df.FindImages(nil)
	assert.NoError(t, err)
	if assert.Equal(t, 1, len(images)) {
		assert.Equal(t, "gcr.io/image-a", images[0].String())
	}
}

func TestFindImagesBadImageName(t *testing.T) {
	// Capital letters aren't allowed in image names
	df := Dockerfile(`FROM gcr.io/imageA`)
	images, err := df.FindImages(nil)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(images))
}

func TestFindImagesMissingImageName(t *testing.T) {
	df := Dockerfile(`FROM`)
	images, err := df.FindImages(nil)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(images))
}

func TestFindImagesWeirdSyntax(t *testing.T) {
	df := Dockerfile(`FROM a b`)
	images, err := df.FindImages(nil)
	assert.NoError(t, err)
	if assert.Equal(t, 1, len(images)) {
		assert.Equal(t, "docker.io/library/a", images[0].String())
	}
}

func TestFindImagesCopyFrom(t *testing.T) {
	df := Dockerfile(`COPY --from=gcr.io/image-a /srcA/package.json /srcB/package.json`)
	images, err := df.FindImages(nil)
	assert.NoError(t, err)
	if assert.Equal(t, 1, len(images)) {
		assert.Equal(t, "gcr.io/image-a", images[0].String())
	}
}

func TestFindImagesWithDefaultArg(t *testing.T) {
	df := Dockerfile(`
ARG TAG="latest"
FROM gcr.io/image-a:${TAG}
`)
	images, err := df.FindImages(nil)
	assert.NoError(t, err)
	if assert.Equal(t, 1, len(images)) {
		assert.Equal(t, "gcr.io/image-a:latest", images[0].String())
	}
}

func TestFindImagesWithNoDefaultArg(t *testing.T) {
	df := Dockerfile(`
ARG TAG
FROM gcr.io/image-a:${TAG}
`)
	images, err := df.FindImages([]string{"TAG=latest"})
	assert.NoError(t, err)
	if assert.Equal(t, 1, len(images)) {
		assert.Equal(t, "gcr.io/image-a:latest", images[0].String())
	}
}

func TestFindImagesWithOverrideArg(t *testing.T) {
	df := Dockerfile(`
ARG TAG="latest"
FROM gcr.io/image-a:${TAG}
`)
	images, err := df.FindImages([]string{"TAG=v2.0.1"})
	assert.NoError(t, err)
	if assert.Equal(t, 1, len(images)) {
		assert.Equal(t, "gcr.io/image-a:v2.0.1", images[0].String())
	}
}

func TestFindImagesWithMount(t *testing.T) {
	// Example from:
	// https://github.com/tilt-dev/tilt/issues/3331
	//
	// The buildkit experimental parser will parse commands
	// like `RUN --mount` that the normal parser won't. So
	// we want to make sure a partial parse succeeds:
	// an bad parse later in the Dockerfile shouldn't interfere
	// with commands further up.
	df := Dockerfile(`
# syntax=docker/dockerfile:experimental

ARG PYTHON2_BASE="python2-base"
FROM ${PYTHON2_BASE}

RUN --mount=type=cache,id=pip,target=/root/.cache/pip pip install python-dateutil
`)
	images, err := df.FindImages(nil)
	assert.NoError(t, err)
	if assert.Equal(t, 1, len(images)) {
		assert.Equal(t, "docker.io/library/python2-base", images[0].String())
	}
}
