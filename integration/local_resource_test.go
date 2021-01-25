// +build integration

package integration

import (
	"io/ioutil"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const cleanupTxt = "cleanup.txt"

func TestLocalResource(t *testing.T) {
	f := newFixture(t, "local_resource")
	defer f.TearDown()

	f.TiltWatch()

	const barServeLogMessage = "Running serve cmd: ./hello.sh bar"
	const readinessProbeSuccessMessage = `"[readiness probe] fake probe success message"`

	require.NoError(t, f.logs.WaitUntilContains("hello! foo #1", 5*time.Second))

	// write a sentinel file for the probe to find and change its result
	require.NoError(t, ioutil.WriteFile(f.testDirPath("probe-success"), nil, 0777))
	require.NoError(t, f.logs.WaitUntilContains("[readiness probe] fake probe success message", 5*time.Second))

	// wait for second resource to start and then ensure that the order in the logs is as expected
	require.NoError(t, f.logs.WaitUntilContains(barServeLogMessage, 5*time.Second))
	curLogs := f.logs.String()
	assert.Greater(t, strings.Index(curLogs, barServeLogMessage), strings.Index(curLogs, readinessProbeSuccessMessage),
		"dependent resource started BEFORE other resource ready")
	require.NoError(t, f.logs.WaitUntilContains("hello! bar #1", 5*time.Second))

	require.NoError(t, os.Remove(f.testDirPath("probe-success")))
	require.NoError(t, f.logs.WaitUntilContains(`[readiness probe] fake probe failure message`, 5*time.Second))

	// send a SIGTERM and make sure Tilt propagates it to its local_resource processes
	require.NoError(t, f.activeTiltUp.process.Signal(syscall.SIGTERM))

	select {
	case <-f.activeTiltUp.done:
	case <-time.After(5 * time.Second):
		t.Fatal("Tilt failed to exit within 5 seconds of SIGTERM")
	}

	// hello.sh writes to cleanup.txt on SIGTERM
	b, err := ioutil.ReadFile(f.testDirPath(cleanupTxt))
	require.NoError(t, err)
	s := string(b)

	require.Contains(t, s, "cleaning up: foo")
	require.Contains(t, s, "cleaning up: bar")
}
