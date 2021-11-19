//go:build !windows
// +build !windows

package cmd

import (
	"os"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStopsBackgroundGrandchildren(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("no bash on windows")
	}
	f := newProcessExecFixture(t)
	defer f.tearDown()

	f.start(`bash -c 'sleep 100 &
echo BACKGROUND $!
'`)
	f.waitForStatus(Done)

	// Since execer uses cmd.Process.Wait instead of cmd.Wait, it doesn't block until the writers have finished.
	// (https://github.com/tilt-dev/tilt/blob/7dad0c01169c7e7825268eff27e96068288280b7/internal/controllers/core/cmd/execer.go#L184)
	// This is probably not currently really a problem for Tilt since the underlying goroutine will still write to the
	// logger's Writer after Wait has returned, but it's the kind of thing that could lead to surprises in the future.
	// (e.g., tests like this, or if we use Cmd to power `local` in the Tiltfile)
	var grandkidPid int
	timeoutInterval := time.Second
	timeout := time.After(time.Second)
	checkInterval := 5 * time.Millisecond
	for {
		lines := strings.Split(f.testWriter.String(), "\n")
		if strings.Contains(lines[1], "BACKGROUND") {
			var err error
			grandkidPid, err = strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(lines[1], "BACKGROUND")))
			require.NoError(t, err)
			break
		}
		select {
		case <-time.After(checkInterval):
		case <-timeout:
			t.Fatalf("timed out after %s waiting for grandkid pid. current output: %q", timeoutInterval, f.testWriter.String())
		}
	}

	grandkid, err := os.FindProcess(grandkidPid)
	require.NoError(t, err)

	// Old unix trick - signal to check if the process is still alive.
	time.Sleep(10 * time.Millisecond)
	err = grandkid.Signal(syscall.SIGCONT)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "process already finished")
	}
}
