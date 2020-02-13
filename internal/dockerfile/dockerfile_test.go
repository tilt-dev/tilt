package dockerfile

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/pkg/model"
)

func TestAllowEntrypoint(t *testing.T) {
	df := Dockerfile(`ENTRYPOINT cat`)
	err := df.ValidateBaseDockerfile()
	if err != nil {
		t.Errorf("Unexpected error %s", err)
	}
}

func TestForbidAdd(t *testing.T) {
	df := Dockerfile(`RUN echo 'hi'
ADD . /go/src`)
	err := df.ValidateBaseDockerfile()
	if err != ErrAddInDockerfile {
		t.Errorf("Expected error %s, actual: %v", ErrAddInDockerfile, err)
	}
}

func TestForbidCopy(t *testing.T) {
	df := Dockerfile(`RUN echo 'hi'
COPY . /go/src`)
	err := df.ValidateBaseDockerfile()
	if err != ErrAddInDockerfile {
		t.Errorf("Expected error %s, actual: %v", ErrAddInDockerfile, err)
	}
}

func TestAllowCopyFrom(t *testing.T) {
	df := Dockerfile(`RUN echo 'hi'
COPY --from=some/other/image . /go/src`)
	err := df.ValidateBaseDockerfile()
	if err != nil {
		t.Errorf("Expected no error, actual: %v", err)
	}
}

func TestForbidAddWithSpaces(t *testing.T) {
	df := Dockerfile(`RUN echo 'hi'
 add . /go/src`)
	err := df.ValidateBaseDockerfile()
	if err != ErrAddInDockerfile {
		t.Errorf("Expected error %s, actual: %v", ErrAddInDockerfile, err)
	}
}

func TestPermitAddInCmd(t *testing.T) {
	df := Dockerfile(`RUN echo ADD`)
	err := df.ValidateBaseDockerfile()
	if err != nil {
		t.Fatal(err)
	}
}

func TestPermitAddInCmd2(t *testing.T) {
	df := Dockerfile(`RUN echo \
ADD`)
	err := df.ValidateBaseDockerfile()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSplitIntoBaseDf(t *testing.T) {
	df := Dockerfile(`

# comment
FROM golang:10

RUN echo hi

ADD . .

RUN echo bye
`)
	a, b, ok := df.SplitIntoBaseDockerfile()
	assert.Equal(t, `

# comment
FROM golang:10

RUN echo hi
`, string(a))
	assert.Equal(t, `ADD . .

RUN echo bye
`, string(b))
	assert.True(t, ok)
}

func TestDeriveSyncs(t *testing.T) {
	df := Dockerfile(`RUN echo 'hi'
COPY foo /bar
ADD /abs/bar /baz
ADD ./beep/boop /blorp`)
	context := "/context/dir"
	syncs, err := df.BUGGY_DeriveSyncs(context)
	if err != nil {
		t.Fatal(err)
	}

	expectedSyncs := []model.Sync{
		model.Sync{
			LocalPath:     path.Join(context, "foo"),
			ContainerPath: "/bar",
		},
		model.Sync{
			LocalPath:     "/abs/bar",
			ContainerPath: "/baz",
		},
		model.Sync{
			LocalPath:     path.Join(context, "beep/boop"),
			ContainerPath: "/blorp",
		},
	}
	assert.Equal(t, len(expectedSyncs), len(syncs))
	for _, s := range expectedSyncs {
		assert.Contains(t, syncs, s)
	}
}

func TestNoAddsToNoSyncs(t *testing.T) {
	df := Dockerfile(`RUN echo 'hi'`)
	syncs, err := df.BUGGY_DeriveSyncs("/context/dir")
	if err != nil {
		t.Fatal(err)
	}
	assert.Empty(t, syncs)
}

func TestFindImages(t *testing.T) {
	df := Dockerfile(`FROM gcr.io/image-a`)
	images, err := df.FindImages()
	assert.NoError(t, err)
	if assert.Equal(t, 1, len(images)) {
		assert.Equal(t, "gcr.io/image-a", images[0].String())
	}
}

func TestFindImagesAsBuilder(t *testing.T) {
	df := Dockerfile(`FROM gcr.io/image-a as builder`)
	images, err := df.FindImages()
	assert.NoError(t, err)
	if assert.Equal(t, 1, len(images)) {
		assert.Equal(t, "gcr.io/image-a", images[0].String())
	}
}

func TestFindImagesBadImageName(t *testing.T) {
	// Capital letters aren't allowed in image names
	df := Dockerfile(`FROM gcr.io/imageA`)
	images, err := df.FindImages()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(images))
}

func TestFindImagesMissingImageName(t *testing.T) {
	df := Dockerfile(`FROM`)
	images, err := df.FindImages()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(images))
}

func TestFindImagesWeirdSyntax(t *testing.T) {
	df := Dockerfile(`FROM a b`)
	images, err := df.FindImages()
	assert.NoError(t, err)
	if assert.Equal(t, 1, len(images)) {
		assert.Equal(t, "docker.io/library/a", images[0].String())
	}
}

func TestFindImagesCopyFrom(t *testing.T) {
	df := Dockerfile(`COPY --from=gcr.io/image-a /srcA/package.json /srcB/package.json`)
	images, err := df.FindImages()
	assert.NoError(t, err)
	if assert.Equal(t, 1, len(images)) {
		assert.Equal(t, "gcr.io/image-a", images[0].String())
	}
}

func TestFindImagesWithArg(t *testing.T) {
	df := Dockerfile(`
ARG TAG="latest"
FROM gcr.io/image-a:${TAG}
`)
	images, err := df.FindImages()
	assert.NoError(t, err)
	if assert.Equal(t, 1, len(images)) {
		assert.Equal(t, "gcr.io/image-a:latest", images[0].String())
	}
}
