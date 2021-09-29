package model

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/container"
)

func TestValidate(t *testing.T) {
	cb := CustomBuild{
		Command: ToHostCmd("exit 0"),
		Deps:    []string{"foo", "bar"},
	}
	it := MustNewImageTarget(container.MustParseSelector("gcr.io/foo/bar")).
		WithBuildDetails(cb)

	assert.Nil(t, it.Validate())
}

func TestDoesNotValidate(t *testing.T) {
	cb := CustomBuild{
		Command: ToHostCmd(""),
		Deps:    []string{"foo", "bar"},
	}
	it := MustNewImageTarget(container.MustParseSelector("gcr.io/foo/bar")).
		WithBuildDetails(cb)

	assert.Error(t, it.Validate())
}
