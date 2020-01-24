//+build integration

package integration

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"github.com/windmilleng/tilt/tools/devlog"

	"github.com/stretchr/testify/require"
)

const cleanupTxt = "cleanup.txt"

var runNum = 1

func TestLocalResourceCleanup(t *testing.T) {
	f := newFixture(t, "local_resource")
	defer f.TearDown()

	devlog.Logf("starting run %d", runNum)

	runNum++

	defer func() {
		b, err := exec.Command("tail", "/tmp/tilt-log").CombinedOutput()
		if err != nil {
			fmt.Printf("error getting tilt-log: %v\n", err)
			return
		}
		fmt.Printf("tail of tilt-log:\n%s\n", b)
	}()

	defer func() {
		_ = os.Remove(f.testDirPath(cleanupTxt))
	}()

	f.TiltWatch()

	require.NoError(t, f.logs.WaitUntilContains("hello! foo #1", 5*time.Second))
	require.NoError(t, f.logs.WaitUntilContains("hello! bar #1", 5*time.Second))

	// send a SIGTERM and make sure Tilt propagates it to its local_resource processes

	require.NoError(t, f.activeTiltUp.process.Signal(syscall.SIGTERM))

	timeout := 2 * time.Second

	select {
	case <-f.activeTiltUp.done:
	case <-time.After(timeout):
		t.Fatalf("Tilt failed to exit within %s of SIGTERM", timeout.String())
	}

	// hello.sh writes to cleanup.txt on SIGTERM
	b, err := ioutil.ReadFile(f.testDirPath(cleanupTxt))
	require.NoError(t, err, "hello.sh did not execute cleanup function")
	s := string(b)

	require.Contains(t, s, "cleaning up: foo")
	require.Contains(t, s, "cleaning up: bar")
}
