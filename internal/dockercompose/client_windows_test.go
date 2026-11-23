//go:build windows

package dockercompose

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"
	"unsafe"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

// On Windows `docker compose` is docker.exe running the docker-compose.exe
// plugin. Canceling a compose command has to stop the plugin too, or every
// Tiltfile reload leaks the `events --json` watcher's plugin process (#6870).
// cmd.exe running ping stands in for docker.exe running the plugin.
func TestDCCommandCancelStopsChildProcesses(t *testing.T) {
	c := &cmdDCClient{mu: &sync.Mutex{}, initCmd: &sync.Once{}, composeCmd: []string{"cmd.exe"}}
	c.initCmd.Do(func() {})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := c.dcCommand(ctx, []string{"/d", "/c", "ping -n 60 127.0.0.1"})
	require.NoError(t, cmd.Start())

	var child uint32
	require.Eventually(t, func() bool {
		child = childProcessOf(t, uint32(cmd.Process.Pid), "PING.EXE")
		return child != 0
	}, 10*time.Second, 50*time.Millisecond, "cmd.exe never started its child")
	h, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, child)
	require.NoError(t, err)
	defer windows.CloseHandle(h)
	defer windows.TerminateProcess(h, 1) // never leave it behind, whatever the test says

	cancel()
	_ = cmd.Wait()

	event, err := windows.WaitForSingleObject(h, 10_000)
	require.NoError(t, err)
	require.Equal(t, uint32(windows.WAIT_OBJECT_0), event, "the child of a canceled compose command is still running")
}

// childProcessOf is the pid of parent's child named exe, 0 when there is none.
// cmd.exe also has a conhost.exe child, which goes when cmd.exe does.
func childProcessOf(t *testing.T, parent uint32, exe string) uint32 {
	t.Helper()
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	require.NoError(t, err)
	defer windows.CloseHandle(snapshot)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		if entry.ParentProcessID == parent && strings.EqualFold(windows.UTF16ToString(entry.ExeFile[:]), exe) {
			return entry.ProcessID
		}
	}
	return 0
}
