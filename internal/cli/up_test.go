package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/model"
)

func TestProdSailUrl(t *testing.T) {
	su, _ := provideSailURL(model.SailModeProd)
	assert.Equal(t, "https://sail.tilt.dev/", su.Http().String())
	assert.Equal(t, "wss://sail.tilt.dev/", su.Ws().String())
}

func TestStagingSailUrl(t *testing.T) {
	su, _ := provideSailURL(model.SailModeStaging)
	assert.Equal(t, "https://sail-staging.tilt.dev/", su.Http().String())
	assert.Equal(t, "wss://sail-staging.tilt.dev/", su.Ws().String())
}

func TestLocalSailUrl(t *testing.T) {
	su, _ := provideSailURL(model.SailModeLocal)
	assert.Equal(t, "http://localhost:10450/", su.Http().String())
	assert.Equal(t, "ws://localhost:10450/", su.Ws().String())
}
