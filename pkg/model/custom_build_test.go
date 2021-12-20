package model

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func TestValidate(t *testing.T) {
	cb := CustomBuild{
		CmdImageSpec: v1alpha1.CmdImageSpec{Args: ToHostCmd("exit 0").Argv},
		Deps:         []string{"foo", "bar"},
	}
	it := MustNewImageTarget(container.MustParseSelector("gcr.io/foo/bar")).
		WithBuildDetails(cb)

	assert.Nil(t, it.Validate())
}

func TestDoesNotValidate(t *testing.T) {
	cb := CustomBuild{
		CmdImageSpec: v1alpha1.CmdImageSpec{},
		Deps:         []string{"foo", "bar"},
	}
	it := MustNewImageTarget(container.MustParseSelector("gcr.io/foo/bar")).
		WithBuildDetails(cb)

	assert.Error(t, it.Validate())
}
