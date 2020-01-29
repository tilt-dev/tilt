//+build integration

package integration

import (
	"fmt"
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

	err = f.tilt.Args([]string{"bar"}, f.LogWriter())
	if err != nil {
		// Currently, Tilt starts printing logs before the webserver has bound to a port.
		// If this happens, just sleep for a second and try again.
		duration := 2 * time.Second
		fmt.Printf("Error setting args. Sleeping (%s): %v\n", duration, err)

		time.Sleep(duration)
		err = f.tilt.Args([]string{"bar"}, f.LogWriter())
		require.NoError(t, err)
	}

	err = f.logs.WaitUntilContains("bar run", time.Second)
	require.NoError(t, err)

	require.NotContains(t, f.logs.String(), "foo run")
}
