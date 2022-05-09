package dockercomposelogstream

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/timecmp"
)

func TestLogActionWriter_SimpleWriter(t *testing.T) {
	st := store.NewTestingStore()
	log := `Attaching to express-redis-docker_app_1, cache
2021-09-08T19:58:01.483005100Z # oO0OoO0OoO0Oo Redis is starting oO0OoO0OoO0Oo
2021-09-08T19:58:01.483027300Z # Redis version=5.0.7, bits=64, commit=00000000, modified=0, pid=1, just started
`

	writer := &LogActionWriter{
		store: st,
	}
	_, err := writer.Write([]byte(log))
	require.NoError(t, err)

	actions := st.Actions()
	require.Equal(t, 1, len(actions))

	expected := `# oO0OoO0OoO0Oo Redis is starting oO0OoO0OoO0Oo
# Redis version=5.0.7, bits=64, commit=00000000, modified=0, pid=1, just started
`
	assert.Equal(t, expected, string(actions[0].(store.LogAction).Message()))
}

func TestLogActionWriter_BrokenLine(t *testing.T) {
	st := store.NewTestingStore()
	log1 := `Attaching to express-redis-docker_app_1, cache
2021-09-08T19:58:01.483005100Z # oO0OoO0`
	log2 := `OoO0Oo Redis is starting oO0OoO0OoO0Oo
2021-09-08T19:58:01.483027300Z # Redis version=5.0.7, bits=64, commit=00000000, modified=0, pid=1, just started
`

	writer := &LogActionWriter{
		store: st,
	}
	_, err := writer.Write([]byte(log1))
	require.NoError(t, err)
	_, err = writer.Write([]byte(log2))
	require.NoError(t, err)

	actions := st.Actions()
	require.Equal(t, 2, len(actions))

	expected1 := `# oO0OoO0`
	assert.Equal(t, expected1, string(actions[0].(store.LogAction).Message()))

	expected2 := `OoO0Oo Redis is starting oO0OoO0OoO0Oo
# Redis version=5.0.7, bits=64, commit=00000000, modified=0, pid=1, just started
`
	assert.Equal(t, expected2, string(actions[1].(store.LogAction).Message()))
}

func TestLogActionWriter_SinceFilter(t *testing.T) {
	st := store.NewTestingStore()
	log := `Attaching to express-redis-docker_app_1, cache
2021-09-08T19:58:01.483005100Z # oO0OoO0OoO0Oo Redis is starting oO0OoO0OoO0Oo
2021-09-16T19:58:01.483027300Z # Redis version=5.0.7, bits=64, commit=00000000, modified=0, pid=1, just started
`

	writer := &LogActionWriter{
		store: st,
		// since is exclusive, so the first line should not appear
		since: time.Date(2021, 9, 8, 19, 58, 1, 483005100, time.UTC),
	}
	_, err := writer.Write([]byte(log))
	require.NoError(t, err)

	actions := st.Actions()
	require.Equal(t, 1, len(actions))

	expected := "# Redis version=5.0.7, bits=64, commit=00000000, modified=0, pid=1, just started\n"
	require.Equal(t, expected, string(actions[0].(store.LogAction).Message()))
	timecmp.RequireTimeEqual(t,
		time.Date(2021, 9, 16, 19, 58, 1, 483027300, time.UTC),
		writer.LastLogTime())
}

func TestLogActionWriter_v2DateFormat(t *testing.T) {
	st := store.NewTestingStore()
	// N.B. there is a single space at the beginning of each _app_ log line before the timestamp w/ Compose v2
	log := `Attaching to express-redis-docker_app_1
 2021-09-08T19:58:01.483005100Z # oO0OoO0OoO0Oo Redis is starting oO0OoO0OoO0Oo
 2021-09-16T19:58:01.483027300Z # Redis version=5.0.7, bits=64, commit=00000000, modified=0, pid=1, just started
express-redis-docker_app_1 exited with code 0
`

	writer := &LogActionWriter{
		store: st,
	}
	_, err := writer.Write([]byte(log))
	require.NoError(t, err)

	actions := st.Actions()
	require.Equal(t, 1, len(actions))

	expected := `# oO0OoO0OoO0Oo Redis is starting oO0OoO0OoO0Oo
# Redis version=5.0.7, bits=64, commit=00000000, modified=0, pid=1, just started
express-redis-docker_app_1 exited with code 0
`
	assert.Equal(t, expected, string(actions[0].(store.LogAction).Message()))
}
