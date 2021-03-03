/*
Copyright 2020 The Tilt Dev Authors

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

package v1alpha1

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource/resourcestrategy"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Cmd
// +k8s:openapi-gen=true
type Cmd struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CmdSpec   `json:"spec,omitempty"`
	Status CmdStatus `json:"status,omitempty"`
}

// CmdList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CmdList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Cmd `json:"items"`
}

// CmdSpec defines the desired state of Cmd
type CmdSpec struct {
	// Command-line arguments. Must have length at least 1.
	Args []string `json:"args,omitempty"`

	// Process working directory.
	//
	// If the working directory is not specified, the command is run
	// in the default Tilt working directory.
	//
	// +optional
	Dir string `json:"dir,omitempty"`

	// Additional variables process environment.
	//
	// Expressed as a C-style array of strings of the form ["KEY1=VALUE1", "KEY2=VALUE2", ...].
	//
	// Environment variables are layered on top of the environment variables
	// that Tilt runs with.
	//
	// +optional
	Env []string `json:"env,omitempty"`

	// Periodic probe of service readiness.
	//
	// +optional
	ReadinessProbe *Probe `json:"readinessProbe,omitempty"`
}

var _ resource.Object = &Cmd{}
var _ resourcestrategy.Validater = &Cmd{}

func (in *Cmd) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *Cmd) NamespaceScoped() bool {
	return false
}

func (in *Cmd) New() runtime.Object {
	return &Cmd{}
}

func (in *Cmd) NewList() runtime.Object {
	return &CmdList{}
}

func (in *Cmd) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "core.tilt.dev",
		Version:  "v1alpha1",
		Resource: "cmds",
	}
}

func (in *Cmd) IsStorageVersion() bool {
	return true
}

func (in *Cmd) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &CmdList{}

func (in *CmdList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// CmdStatus defines the observed state of Cmd
//
// Based loosely on ContainerStatus in Kubernetes
type CmdStatus struct {
	// Details about a waiting process.
	// +optional
	Waiting *CmdStateWaiting `json:"waiting,omitempty"`

	// Details about a running process.
	// +optional
	Running *CmdStateRunning `json:"running,omitempty"`

	// Details about a terminated process.
	// +optional
	Terminated *CmdStateTerminated `json:"terminated,omitempty"`

	// Specifies whether the command has passed its readiness probe.
	//
	// Terminating the command does not change its Ready state.
	//
	// Is always true when no readiness probe is defined.
	//
	// +optional
	Ready bool `json:"ready,omitempty"`
}

// CmdStateWaiting is a waiting state of a local command.
type CmdStateWaiting struct {
	// (brief) reason the process is not yet running.
	// +optional
	Reason string `json:"reason,omitempty"`
}

// CmdStateRunning is a running state of a local command.
type CmdStateRunning struct {
	// The process id of the command.
	PID int32 `json:"pid"`

	// Time at which the command was last started.
	StartedAt metav1.Time `json:"startedAt,omitempty"`
}

// CmdStateTerminated is a terminated state of a local command.
type CmdStateTerminated struct {
	// The process id of the command.
	PID int32 `json:"pid"`

	// Exit status from the last termination of the command
	ExitCode int32 `json:"exitCode"`

	// Time at which previous execution of the command started
	StartedAt metav1.Time `json:"startedAt,omitempty"`

	// Time at which the command last terminated
	FinishedAt metav1.Time `json:"finishedAt,omitempty"`

	// (brief) reason the process is terminated
	// +optional
	Reason string `json:"reason,omitempty"`
}

// Cmd implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &Cmd{}

func (in *Cmd) GetStatus() resource.StatusSubResource {
	return in.Status
}

// CmdStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &CmdStatus{}

func (in CmdStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*Cmd).Status = in
}
