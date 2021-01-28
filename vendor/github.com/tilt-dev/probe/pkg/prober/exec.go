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

	"k8s.io/klog/v2"
	"k8s.io/utils/exec"
)

const (
	// TODO(milas): consider adding back limited output
	maxReadLength = 10 * 1 << 10 // 10KB
)

// osExecRunner is the default runner that's a shim around stdlib os/exec.
//
// A global instance is used to avoid creating an instance for every probe (it is Goroutine safe).
var osExecRunner = exec.New()

// NewExecProber creates an ExecProber.
func NewExecProber() ExecProber {
	return execProber{
		runner: osExecRunner,
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

type execProber struct {
	runner exec.Interface
}

// Probe executes a command to check service status.
func (pr execProber) Probe(ctx context.Context, name string, args ...string) (Result, string, error) {
	cmd := pr.runner.CommandContext(ctx, name, args...)
	data, err := cmd.CombinedOutput()

	klog.V(4).Infof("Exec probe response: %q", string(data))
	if err != nil {
		exit, ok := err.(exec.ExitError)
		if ok {
			if exit.ExitStatus() == 0 {
				return Success, string(data), nil
			}
			return Failure, string(data), nil
		}

		return Unknown, "", err
	}
	return Success, string(data), nil
}
