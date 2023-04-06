package dockercomposelogstream

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
)

func TestDeepEqual(t *testing.T) {
	c1 := &ContainerInfo{ID: "x-1"}
	c1b := &ContainerInfo{ID: "x-1"}
	c2 := &ContainerInfo{ID: "x-2"}
	assert.False(t, apicmp.DeepEqual(c1, c2))
	assert.True(t, apicmp.DeepEqual(c1, c1b))
}
