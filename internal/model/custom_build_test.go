package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/container"
)

func TestMaybeFastBuildInfo(t *testing.T) {
	fb := FastBuild{
		BaseDockerfile: "FROM alpine",
		Entrypoint:     Cmd{[]string{"echo", "hi"}},
	}
	cb := CustomBuild{
		Command: "true",
		Deps:    []string{"foo", "bar"},
		Fast:    &fb,
	}
	it := ImageTarget{
		BuildDetails: cb,
	}
	bi := it.MaybeFastBuildInfo()
	assert.Equal(t, fb, *bi)

	it = ImageTarget{
		BuildDetails: fb,
	}
	bi = it.MaybeFastBuildInfo()
	assert.Equal(t, fb, *bi)

	it = ImageTarget{
		BuildDetails: StaticBuild{},
	}
	bi = it.MaybeFastBuildInfo()
	assert.Nil(t, bi)
}

func TestValidate(t *testing.T) {
	cb := CustomBuild{
		Command: "true",
		Deps:    []string{"foo", "bar"},
	}
	it := ImageTarget{
		Ref:          container.MustParseNamed("gcr.io/foo/bar"),
		BuildDetails: cb,
	}

	assert.Nil(t, it.Validate())
}

func TestDoesNotValidate(t *testing.T) {
	cb := CustomBuild{
		Command: "",
		Deps:    []string{"foo", "bar"},
	}
	it := ImageTarget{
		Ref:          container.MustParseNamed("gcr.io/foo/bar"),
		BuildDetails: cb,
	}

	assert.Error(t, it.Validate())
}
