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

import "context"

// Result is a string used to indicate service status.
type Result string

const (
	// Success indicates that the service is healthy.
	Success Result = "success"
	// Warning indicates that the service is healthy but additional diagnostic information might be attached.
	Warning Result = "warning"
	// Failure indicates that the service is not healthy.
	Failure Result = "failure"
	// Unknown indicates that the prober was unable to determine the service status due to an internal issue.
	//
	// An Unknown result should also include an error in the Prober return values.
	Unknown Result = "unknown"
)

// Prober performs a check to determine a service status.
type Prober interface {
	// Probe executes a single status check.
	//
	// result is the current service status
	// output is optional info from the probe (such as a process stdout or HTTP response)
	// err indicates an issue with the probe itself and that the result should be ignored
	Probe(ctx context.Context) (result Result, output string, err error)
}

// ProberFunc is a functional version of Prober.
type ProberFunc func(ctx context.Context) (Result, string, error)

// Probe executes a single status check.
//
// result is the current service status
// output is optional info from the probe (such as a process stdout or HTTP response)
// err indicates an issue with the probe itself and that the result should be ignored
func (f ProberFunc) Probe(ctx context.Context) (Result, string, error) {
	return f(ctx)
}
