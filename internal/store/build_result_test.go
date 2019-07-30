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

func TestOneAndOnlyLiveUpdatedContainerID(t *testing.T) {
	set := BuildResultSet{
		imageID("a"): BuildResult{LiveUpdatedContainerIDs: []container.ID{"cA"}},
		imageID("b"): BuildResult{LiveUpdatedContainerIDs: []container.ID{"cB"}},
	}
	assert.Equal(t, "", string(set.OneAndOnlyLiveUpdatedContainerID()))

	set = BuildResultSet{
		imageID("a"): BuildResult{LiveUpdatedContainerIDs: []container.ID{"cA"}},
		imageID("b"): BuildResult{LiveUpdatedContainerIDs: []container.ID{"cA"}},
		imageID("c"): BuildResult{LiveUpdatedContainerIDs: []container.ID{""}},
	}
	assert.Equal(t, "cA", string(set.OneAndOnlyLiveUpdatedContainerID()))
}
