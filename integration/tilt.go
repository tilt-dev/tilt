package integration

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type TiltDriver struct {
	Environ map[string]string
}

func NewTiltDriver() *TiltDriver {
	return &TiltDriver{
		Environ: make(map[string]string),
	}
}

func (d *TiltDriver) cmd(args []string, out io.Writer) *exec.Cmd {
	cmd := exec.Command("tilt", args...)
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.Env = os.Environ()
	for k, v := range d.Environ {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
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

func (d *TiltDriver) Up(args []string, out io.Writer) (*TiltUpResponse, error) {
	mandatoryArgs := []string{"up",
		// Can't attach a HUD or install browsers in headless mode
		"--hud=false",
		"--no-browser",

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
