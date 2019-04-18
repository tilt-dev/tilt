package store

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/model"
)

func imageID(s string) model.TargetID {
	return model.TargetID{
		Type: model.TargetTypeImage,
		Name: model.TargetName(s),
	}
}

func TestOneAndOnlyContainerID(t *testing.T) {
	set := BuildResultSet{
		imageID("a"): BuildResult{ContainerID: "cA"},
		imageID("b"): BuildResult{ContainerID: "cB"},
	}
	assert.Equal(t, "", string(set.OneAndOnlyContainerID()))

	set = BuildResultSet{
		imageID("a"): BuildResult{ContainerID: "cA"},
		imageID("b"): BuildResult{ContainerID: "cA"},
		imageID("c"): BuildResult{ContainerID: ""},
	}
	assert.Equal(t, "cA", string(set.OneAndOnlyContainerID()))
}
