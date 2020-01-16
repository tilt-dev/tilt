//+build integration

package integration

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTiltArgs(t *testing.T) {
	f := newFixture(t, "tilt_args")
	defer f.TearDown()

	f.tiltArgs = []string{"foo"}

	f.TiltWatch()

	err := f.logs.WaitUntilContains("foo run", time.Second)
	require.NoError(t, err)

	f.logs.Reset()

	err = f.tilt.Args([]string{"bar"}, f.logs)
	require.NoError(t, err)

	err = f.logs.WaitUntilContains("bar run", time.Second)
	require.NoError(t, err)

	require.NotContains(t, f.logs.String(), "foo run")
}
