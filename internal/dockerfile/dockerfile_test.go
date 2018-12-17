package dockerfile

import (
	"path"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/model"
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

func TestDeriveMounts(t *testing.T) {
	df := Dockerfile(`RUN echo 'hi'
COPY foo /bar
ADD /abs/bar /baz
ADD ./beep/boop /blorp`)
	context := "/context/dir"
	mounts, err := df.DeriveMounts(context)
	if err != nil {
		t.Fatal(err)
	}

	expectedMounts := []model.Mount{
		model.Mount{
			LocalPath:     path.Join(context, "foo"),
			ContainerPath: "/bar",
		},
		model.Mount{
			LocalPath:     "/abs/bar",
			ContainerPath: "/baz",
		},
		model.Mount{
			LocalPath:     path.Join(context, "beep/boop"),
			ContainerPath: "/blorp",
		},
	}
	assert.Equal(t, len(expectedMounts), len(mounts))
	for _, m := range expectedMounts {
		assert.Contains(t, mounts, m)
	}
}

func TestNoAddsToNoMounts(t *testing.T) {
	df := Dockerfile(`RUN echo 'hi'`)
	mounts, err := df.DeriveMounts("/context/dir")
	if err != nil {
		t.Fatal(err)
	}
	assert.Empty(t, mounts)
}
