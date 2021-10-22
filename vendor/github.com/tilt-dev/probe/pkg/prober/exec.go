/*
Copyright 2015 The Kubernetes Authors.
Modified 2021 Windmill Engineering.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package prober

import (
	"context"
	"os/exec"
	"syscall"

	"k8s.io/klog/v2"

	"github.com/tilt-dev/probe/internal/procutil"
)

const (
	// TODO(milas): consider adding back limited output
	maxReadLength = 10 * 1 << 10 // 10KB
)

// NewExecProber creates an ExecProber.
func NewExecProber() ExecProber {
	return execProber{
		excer: realExecer,
	}
}

// ExecProber executes a command to check a service status.
type ExecProber interface {
	// Probe executes a command to check a service status.
	//
	// If the process terminates with any exit code besides 0, Failure will be returned.
	// The merged result of stdout + stderr are returned as output.
	Probe(ctx context.Context, name string, args ...string) (Result, string, error)
}

type processExecer func(ctx context.Context, name string, args ...string) (exitCode int, out []byte, err error)

type execProber struct {
	excer processExecer
}

// Probe executes a command to check service status.
func (pr execProber) Probe(ctx context.Context, name string, args ...string) (Result, string, error) {
	exitCode, out, err := pr.excer(ctx, name, args...)
	klog.V(4).Infof("Exec probe response (exit code %d): %s", exitCode, string(out))
	if err != nil {
		return Unknown, "", err
	}
	if exitCode != 0 {
		return Failure, string(out), nil
	}
	return Success, string(out), nil
}

var realExecer = func(ctx context.Context, name string, args ...string) (int, []byte, error) {
	// N.B. we don't use CommandContext because we want slightly different semantics to kill the
	// 	entire process group without introducing a race between us and Go stdlib
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	procutil.SetOptNewProcessGroup(cmd.SysProcAttr)

	// we want the (partial) I/O even if we kill the process group due to context deadline exceeded
	// so they're managed manually vs using something like CombinedOutput()
	//
	// managing I/O properly when not using the higher-level stdlib functions is error-prone :)
	//
	// there are two cases here:
	// 	* Cmd::Wait() returns -> Go stdlib ensures that I/O is fully copied, we don't need to do anything
	//  * Context::Done() is hit -> we killed the process, I/O might be in an incomplete state, but that's
	// 		acceptable given that we terminated it early anyway (it's only useful for debugging regardless)
	var output threadSafeBuffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Start(); err != nil {
		return -1, nil, err
	}

	procExitCh := make(chan error, 1)
	go func() {
		// this WILL block on child processes, but that's ok since we handle the timeout termination below
		// and it's preferable vs using Process::Wait() since that complicates I/O handling (Cmd::Wait() will
		// ensure all I/O is complete before returning)
		err := cmd.Wait()
		if err != nil {
			procExitCh <- err
		} else {
			procExitCh <- nil
		}
		close(procExitCh)
	}()

	select {
	case <-ctx.Done():
		procutil.KillProcessGroup(cmd)
		return -1, output.Bytes(), nil
	case err := <-procExitCh:
		if err != nil {
			if exit, ok := err.(*exec.ExitError); ok {
				return exit.ExitCode(), output.Bytes(), nil
			}
			return -1, output.Bytes(), err
		}
		return 0, output.Bytes(), nil
	}
}
