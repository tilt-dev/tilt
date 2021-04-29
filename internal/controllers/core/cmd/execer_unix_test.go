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

	lines := strings.Split(f.testWriter.String(), "\n")
	assert.Contains(t, lines[1], "BACKGROUND")
	grandkidPid, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(lines[1], "BACKGROUND")))
	require.NoError(t, err)

	grandkid, err := os.FindProcess(grandkidPid)
	require.NoError(t, err)

	// Old unix trick - signal to check if the process is still alive.
	time.Sleep(10 * time.Millisecond)
	err = grandkid.Signal(syscall.SIGCONT)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "process already finished")
	}
}
