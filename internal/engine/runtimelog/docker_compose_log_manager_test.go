package runtimelog

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/store"
)

func TestSimpleWriter(t *testing.T) {
	st := store.NewTestingStore()
	log := `Attaching to express-redis-docker_app_1, cache
cache    | # oO0OoO0OoO0Oo Redis is starting oO0OoO0OoO0Oo
cache    | # Redis version=5.0.7, bits=64, commit=00000000, modified=0, pid=1, just started
`

	writer := &DockerComposeLogActionWriter{
		store:             st,
		isStartingNewLine: true,
	}
	writer.Write([]byte(log))

	actions := st.Actions()
	require.Equal(t, 1, len(actions))

	expected := `# oO0OoO0OoO0Oo Redis is starting oO0OoO0OoO0Oo
# Redis version=5.0.7, bits=64, commit=00000000, modified=0, pid=1, just started
`
	assert.Equal(t, expected, string(actions[0].(DockerComposeLogAction).Message()))
}

func TestBrokenLine(t *testing.T) {
	st := store.NewTestingStore()
	log1 := `Attaching to express-redis-docker_app_1, cache
cache    | # oO0OoO0`
	log2 := `OoO0Oo Redis is starting oO0OoO0OoO0Oo
cache    | # Redis version=5.0.7, bits=64, commit=00000000, modified=0, pid=1, just started
`

	writer := &DockerComposeLogActionWriter{
		store:             st,
		isStartingNewLine: true,
	}
	writer.Write([]byte(log1))
	writer.Write([]byte(log2))

	actions := st.Actions()
	require.Equal(t, 2, len(actions))

	expected1 := `# oO0OoO0`
	assert.Equal(t, expected1, string(actions[0].(DockerComposeLogAction).Message()))

	expected2 := `OoO0Oo Redis is starting oO0OoO0OoO0Oo
# Redis version=5.0.7, bits=64, commit=00000000, modified=0, pid=1, just started
`
	assert.Equal(t, expected2, string(actions[1].(DockerComposeLogAction).Message()))
}
