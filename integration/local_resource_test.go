//go:build integration
// +build integration

package integration

import (
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

	removeTestFiles := func() {
		require.NoError(t, os.RemoveAll(f.testDirPath(cleanupTxt)))
		require.NoError(t, os.RemoveAll(f.testDirPath("greeting")))
		require.NoError(t, os.RemoveAll(f.testDirPath("probe-success")))
	}
	removeTestFiles()
	t.Cleanup(removeTestFiles)

	f.TiltUp()

	const barServeLogMessage = "Running cmd: ./hello.sh bar"
	const readinessProbeSuccessMessage = `[readiness probe: success] fake probe success message`

	f.logs.AssertEventuallyContains(t, "hello! foo #1", 5*time.Second)

	// write a sentinel file for the probe to find and change its result
	if assert.NoError(t, os.WriteFile(f.testDirPath("probe-success"), nil, 0777)) {
		f.logs.AssertEventuallyContains(t, readinessProbeSuccessMessage, 5*time.Second)
	}

	// wait for second resource to start and then ensure that the order in the logs is as expected
	f.logs.AssertEventuallyContains(t, barServeLogMessage, 5*time.Second)
	curLogs := f.logs.String()
	assert.Greater(t, strings.Index(curLogs, barServeLogMessage), strings.Index(curLogs, readinessProbeSuccessMessage),
		"dependent resource started BEFORE other resource ready")
	f.logs.AssertEventuallyContains(t, "hello! bar #1", 5*time.Second)

	// trigger a service restart by changing a watched file
	if assert.NoError(t, os.WriteFile(f.testDirPath("greeting"), []byte("hola"), 0777)) {
		f.logs.AssertEventuallyContains(t, "hola! foo #1", 5*time.Second)
	}

	// force the probe into a failure state
	if assert.NoError(t, os.Remove(f.testDirPath("probe-success"))) {
		f.logs.AssertEventuallyContains(t, `[readiness probe: failure] fake probe failure message`, 5*time.Second)
	}

	// send a SIGTERM and make sure Tilt propagates it to its local_resource processes
	require.NoError(t, f.activeTiltUp.process.Signal(syscall.SIGTERM))

	select {
	case <-f.activeTiltUp.done:
	case <-time.After(5 * time.Second):
		t.Fatal("Tilt failed to exit within 5 seconds of SIGTERM")
	}

	// hello.sh writes to cleanup.txt on SIGTERM
	b, err := os.ReadFile(f.testDirPath(cleanupTxt))
	if assert.NoError(t, err) {
		s := string(b)
		require.Contains(t, s, "cleaning up: foo")
		require.Contains(t, s, "cleaning up: bar")
	}
}
