//+build integration

package integration

import (
	"io/ioutil"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const cleanupTxt = "cleanup.txt"

func TestLocalResourceCleanup(t *testing.T) {
	f := newFixture(t, "local_resource")
	defer f.TearDown()

	defer func() {
		_ = os.Remove(f.testDirPath(cleanupTxt))
	}()

	f.TiltWatch()

	require.NoError(t, f.logs.WaitUntilContains("hello! foo #1", 5*time.Second))
	require.NoError(t, f.logs.WaitUntilContains("hello! bar #1", 5*time.Second))

	// send a SIGTERM and make sure Tilt propagates it to its local_resource processes

	require.NoError(t, f.activeTiltUp.process.Signal(syscall.SIGTERM))

	select {
	case <-f.activeTiltUp.done:
	case <-time.After(2 * time.Second):
		t.Fatal("Tilt failed to exit within 2 seconds of SIGTERM")
	}

	// hello.sh writes to cleanup.txt on SIGTERM
	b, err := ioutil.ReadFile(f.testDirPath(cleanupTxt))
	require.NoError(t, err)
	s := string(b)

	require.Contains(t, s, "cleaning up: foo")
	require.Contains(t, s, "cleaning up: bar")
}
