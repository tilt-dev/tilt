package model

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/container"
)

func TestEmptyLiveUpdate(t *testing.T) {
	lu, err := NewLiveUpdate(nil, "/base/dir")
	if err != nil {
		t.Fatal(err)
	}
	cb := CustomBuild{
		Command:    "true",
		Deps:       []string{"foo", "bar"},
		LiveUpdate: lu,
	}
	it := ImageTarget{
		BuildDetails: cb,
	}
	bi := it.LiveUpdateInfo()
	assert.True(t, bi.Empty())
}

func TestValidate(t *testing.T) {
	cb := CustomBuild{
		Command: "true",
		Deps:    []string{"foo", "bar"},
	}
	it := MustNewImageTarget(container.MustParseSelector("gcr.io/foo/bar")).
		WithBuildDetails(cb)

	assert.Nil(t, it.Validate())
}

func TestDoesNotValidate(t *testing.T) {
	cb := CustomBuild{
		Command: "",
		Deps:    []string{"foo", "bar"},
	}
	it := MustNewImageTarget(container.MustParseSelector("gcr.io/foo/bar")).
		WithBuildDetails(cb)

	assert.Error(t, it.Validate())
}
