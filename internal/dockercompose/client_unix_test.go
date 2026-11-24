//go:build !windows

package dockercompose

import (
	"bufio"
	"context"
	"io"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// `docker compose` is the docker CLI running the compose plugin. Canceling a
// compose command has to stop the plugin too, or every Tiltfile reload leaks
// the `events --json` watcher's plugin process (#6870). sh running sleep
// stands in for the CLI running the plugin.
func TestDCCommandCancelStopsChildProcesses(t *testing.T) {
	c := &cmdDCClient{mu: &sync.Mutex{}, initCmd: &sync.Once{}, composeCmd: []string{"sh"}}
	c.initCmd.Do(func() {})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd := c.dcCommand(ctx, []string{"-c", "sleep 60 & echo started; wait"})

	// sleep inherits the write end, so the read end only reaches EOF once
	// sleep is gone too. That holds even while an exited sleep waits to be
	// reaped, which a pid check wouldn't.
	r, w, err := os.Pipe()
	require.NoError(t, err)
	defer r.Close()
	cmd.Stdout = w
	require.NoError(t, cmd.Start())
	w.Close()

	out := bufio.NewReader(r)
	line, err := out.ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, "started\n", line)

	cancel()
	_ = cmd.Wait()

	closed := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, out)
		close(closed)
	}()
	select {
	case <-closed:
	case <-time.After(10 * time.Second):
		t.Fatal("the child of a canceled compose command is still running")
	}
}
