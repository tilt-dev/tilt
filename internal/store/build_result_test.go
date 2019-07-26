package store

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/container"
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
		imageID("a"): BuildResult{ContainerIDs: []container.ID{"cA"}},
		imageID("b"): BuildResult{ContainerIDs: []container.ID{"cB"}},
	}
	assert.Equal(t, "", string(set.OneAndOnlyContainerID()))

	set = BuildResultSet{
		imageID("a"): BuildResult{ContainerIDs: []container.ID{"cA"}},
		imageID("b"): BuildResult{ContainerIDs: []container.ID{"cA"}},
		imageID("c"): BuildResult{ContainerIDs: []container.ID{""}},
	}
	assert.Equal(t, "cA", string(set.OneAndOnlyContainerID()))
}
