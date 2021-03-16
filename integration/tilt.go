package integration

import (
	"fmt"
	"go/build"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type TiltDriver struct {
	Port    int
	Environ map[string]string
}

func NewTiltDriver(t testing.TB) *TiltDriver {
	l, err := net.Listen("tcp", "")
	require.NoError(t, err, "Could not get a free port")
	port := l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close(), "Could not get a free port")
	return NewTiltDriverWithExplicitPort(port)
}

func NewTiltDriverWithExplicitPort(port int) *TiltDriver {
	return &TiltDriver{
		Port:    port,
		Environ: make(map[string]string),
	}
}

func (d *TiltDriver) cmd(args []string, out io.Writer) *exec.Cmd {
	// rely on the Tilt binary in GOPATH that should have been created by `go install` from the
	// fixture to avoid accidentally picking up a system install of tilt with higher precedence
	// on system PATH
	tiltBin := filepath.Join(build.Default.GOPATH, "bin", "tilt")
	cmd := exec.Command(tiltBin, args...)
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Env = os.Environ()
	for k, v := range d.Environ {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	hasPortArg := false
	for _, arg := range args {
		if strings.HasPrefix("--port=", arg) {
			hasPortArg = true
			break
		}
	}
	if !hasPortArg && d.Port > 0 {
		cmd.Env = append(cmd.Env, fmt.Sprintf("TILT_PORT=%d", d.Port))
	}
	return cmd
}

func (d *TiltDriver) DumpEngine(out io.Writer) error {
	cmd := d.cmd([]string{"dump", "engine"}, out)
	return cmd.Run()
}

func (d *TiltDriver) Down(out io.Writer) error {
	cmd := d.cmd([]string{"down"}, out)
	return cmd.Run()
}

func (d *TiltDriver) CI(out io.Writer, args ...string) error {
	cmd := d.cmd(append([]string{
		"ci",

		// Debug logging for integration tests
		"--debug",
		"--klog=1",

		// Even if we're on a debug build, don't start a debug webserver
		"--web-mode=prod",
	}, args...), out)
	return cmd.Run()
}

func (d *TiltDriver) Up(out io.Writer, args ...string) (*TiltUpResponse, error) {
	mandatoryArgs := []string{"up",
		// Can't attach a HUD or install browsers in headless mode
		"--hud=false",

		// Debug logging for integration tests
		"--debug",
		"--klog=1",

		// Even if we're on a debug build, don't start a debug webserver
		"--web-mode=prod",
	}

	cmd := d.cmd(append(mandatoryArgs, args...), out)
	err := cmd.Start()
	if err != nil {
		return nil, err
	}

	ch := make(chan struct{})
	response := &TiltUpResponse{
		done:    ch,
		process: cmd.Process,
	}
	go func() {
		err := cmd.Wait()
		if err != nil {
			response.mu.Lock()
			response.err = err
			response.mu.Unlock()
		}
		close(ch)
	}()
	return response, nil
}

func (d *TiltDriver) Args(args []string, out io.Writer) error {
	cmd := d.cmd(append([]string{"args"}, args...), out)
	return cmd.Run()
}

type TiltUpResponse struct {
	done chan struct{}
	err  error
	mu   sync.Mutex

	process *os.Process
}

func (r *TiltUpResponse) Done() <-chan struct{} {
	return r.done
}

func (r *TiltUpResponse) Err() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.err
}

func (r *TiltUpResponse) Kill() error {
	if r.process == nil {
		return nil
	}
	return r.process.Kill()
}

// Kill the tilt process and print the goroutine/register state.
// Useful if you think Tilt is deadlocked but aren't sure why.
func (r *TiltUpResponse) KillAndDumpThreads() error {
	if r.process == nil {
		return nil
	}

	err := r.process.Signal(syscall.Signal(syscall.SIGINT))
	if err != nil {
		return err
	}

	select {
	case <-r.Done():
	case <-time.After(2 * time.Second):
	}
	return nil
}
