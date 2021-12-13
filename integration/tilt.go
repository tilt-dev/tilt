package integration

import (
	"bufio"
	"bytes"
	"context"
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

type UpCommand string

const (
	UpCommandUp   UpCommand = "up"
	UpCommandDemo UpCommand = "demo"
)

type TiltDriver struct {
	Environ map[string]string

	t    testing.TB
	port int
}

type TiltDriverOption func(t testing.TB, td *TiltDriver)

func TiltDriverUseRandomFreePort(t testing.TB, td *TiltDriver) {
	l, err := net.Listen("tcp", "")
	require.NoError(t, err, "Could not get a free port")
	td.port = l.Addr().(*net.TCPAddr).Port
	require.NoError(t, l.Close(), "Could not get a free port")
}

func NewTiltDriver(t testing.TB, options ...TiltDriverOption) *TiltDriver {
	td := &TiltDriver{
		t:       t,
		Environ: make(map[string]string),
	}
	for _, opt := range options {
		opt(t, td)
	}
	return td
}

func (d *TiltDriver) cmd(ctx context.Context, args []string, out io.Writer) *exec.Cmd {
	// rely on the Tilt binary in GOPATH that should have been created by `go install` from the
	// fixture to avoid accidentally picking up a system install of tilt with higher precedence
	// on system PATH
	tiltBin := filepath.Join(build.Default.GOPATH, "bin", "tilt")
	cmd := exec.CommandContext(ctx, tiltBin, args...)
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Env = os.Environ()
	for k, v := range d.Environ {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	if d.port > 0 {
		for _, arg := range args {
			if strings.HasPrefix("--port=", arg) {
				d.t.Fatalf("Cannot specify port argument when using automatic port mode: %s", arg)
			}
		}
		if _, ok := d.Environ["TILT_PORT"]; ok {
			d.t.Fatal("Cannot specify TILT_PORT environment variable when using automatic port mode")
		}
		cmd.Env = append(cmd.Env, fmt.Sprintf("TILT_PORT=%d", d.port))
	}
	return cmd
}

func (d *TiltDriver) DumpEngine(ctx context.Context, out io.Writer) error {
	cmd := d.cmd(ctx, []string{"dump", "engine"}, out)
	return cmd.Run()
}

func (d *TiltDriver) Down(ctx context.Context, out io.Writer) error {
	cmd := d.cmd(ctx, []string{"down"}, out)
	return cmd.Run()
}

func (d *TiltDriver) CI(ctx context.Context, out io.Writer, args ...string) error {
	cmd := d.cmd(ctx, append([]string{
		"ci",

		// Debug logging for integration tests
		"--debug",
		"--klog=1",

		// Even if we're on a debug build, don't start a debug webserver
		"--web-mode=prod",
	}, args...), out)
	return cmd.Run()
}

func (d *TiltDriver) Up(ctx context.Context, command UpCommand, out io.Writer, args ...string) (*TiltUpResponse, error) {
	if command == "" {
		command = UpCommandUp
	}
	mandatoryArgs := []string{string(command),
		// Can't attach a HUD or install browsers in headless mode
		"--hud=false",

		// Debug logging for integration tests
		"--debug",
		"--klog=1",

		// Even if we're on a debug build, don't start a debug webserver
		"--web-mode=prod",
	}

	cmd := d.cmd(ctx, append(mandatoryArgs, args...), out)
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

func (d *TiltDriver) Args(ctx context.Context, args []string, out io.Writer) error {
	cmd := d.cmd(ctx, append([]string{"args"}, args...), out)
	return cmd.Run()
}

func (d *TiltDriver) APIResources(ctx context.Context) ([]string, error) {
	var out bytes.Buffer
	cmd := d.cmd(ctx, []string{"api-resources", "-o=name"}, &out)
	err := cmd.Run()
	if err != nil {
		return nil, err
	}
	var resources []string
	s := bufio.NewScanner(&out)
	for s.Scan() {
		resources = append(resources, s.Text())
	}
	return resources, nil
}

func (d *TiltDriver) Get(ctx context.Context, apiType string, names ...string) ([]byte, error) {
	var out bytes.Buffer
	args := append([]string{"get", "-o=json", apiType}, names...)
	cmd := d.cmd(ctx, args, &out)
	err := cmd.Run()
	return out.Bytes(), err
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

// TriggerExit sends a SIGTERM to the `tilt up` process to give it a chance to exit normally.
//
// If the signal cannot be sent or 2 seconds have elapsed, it will be forcibly killed with SIGKILL.
func (r *TiltUpResponse) TriggerExit() error {
	if r.process == nil {
		return nil
	}

	if err := r.process.Signal(syscall.SIGTERM); err != nil {
		return r.process.Kill()
	}

	select {
	case <-r.Done():
	case <-time.After(2 * time.Second):
		return r.process.Kill()
	}

	return nil
}

// Kill the tilt process and print the goroutine/register state.
// Useful if you think Tilt is deadlocked but aren't sure why.
func (r *TiltUpResponse) KillAndDumpThreads() error {
	if r.process == nil {
		return nil
	}

	err := r.process.Signal(syscall.SIGINT)
	if err != nil {
		return err
	}

	select {
	case <-r.Done():
	case <-time.After(2 * time.Second):
	}
	return nil
}
