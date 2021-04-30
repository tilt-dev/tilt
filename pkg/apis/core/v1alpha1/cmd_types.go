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

// Cmd represents a process on the host machine.
//
// When the process exits, we will make a best-effort attempt
// (within OS limitations) to kill any spawned descendant processes.
//
// +k8s:openapi-gen=true
type Cmd struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   CmdSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status CmdStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// CmdList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CmdList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []Cmd `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// CmdSpec defines how to run a local command.
type CmdSpec struct {
	// Command-line arguments. Must have length at least 1.
	Args []string `json:"args,omitempty" protobuf:"bytes,1,rep,name=args"`

	// Process working directory.
	//
	// If the working directory is not specified, the command is run
	// in the default Tilt working directory.
	//
	// +optional
	Dir string `json:"dir,omitempty" protobuf:"bytes,2,opt,name=dir"`

	// Additional variables process environment.
	//
	// Expressed as a C-style array of strings of the form ["KEY1=VALUE1", "KEY2=VALUE2", ...].
	//
	// Environment variables are layered on top of the environment variables
	// that Tilt runs with.
	//
	// +optional
	Env []string `json:"env,omitempty" protobuf:"bytes,3,rep,name=env"`

	// Periodic probe of service readiness.
	//
	// +optional
	ReadinessProbe *Probe `json:"readinessProbe,omitempty" protobuf:"bytes,4,opt,name=readinessProbe"`

	// Indicates objects that can trigger a restart of this command.
	//
	// When a restart is triggered, Tilt will try to gracefully shutdown any
	// currently running process, waiting for it to exit before starting a new
	// process. If the process doesn't shutdown within the allotted time, Tilt
	// will kill the process abruptly.
	//
	// Restarts can happen even if the command is already done.
	//
	// Logs of the current process after the restart are discarded.
	RestartOn *RestartOnSpec `json:"restartOn,omitempty" protobuf:"bytes,5,opt,name=restartOn"`
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
		Group:    "tilt.dev",
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
	Waiting *CmdStateWaiting `json:"waiting,omitempty" protobuf:"bytes,1,opt,name=waiting"`

	// Details about a running process.
	// +optional
	Running *CmdStateRunning `json:"running,omitempty" protobuf:"bytes,2,opt,name=running"`

	// Details about a terminated process.
	// +optional
	Terminated *CmdStateTerminated `json:"terminated,omitempty" protobuf:"bytes,3,opt,name=terminated"`

	// Specifies whether the command has passed its readiness probe.
	//
	// Terminating the command does not change its Ready state.
	//
	// Is always true when no readiness probe is defined.
	//
	// +optional
	Ready bool `json:"ready,omitempty" protobuf:"varint,4,opt,name=ready"`
}

// CmdStateWaiting is a waiting state of a local command.
type CmdStateWaiting struct {
	// (brief) reason the process is not yet running.
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,1,opt,name=reason"`
}

// CmdStateRunning is a running state of a local command.
type CmdStateRunning struct {
	// The process id of the command.
	PID int32 `json:"pid" protobuf:"varint,1,opt,name=pid"`

	// Time at which the command was last started.
	StartedAt metav1.MicroTime `json:"startedAt,omitempty" protobuf:"bytes,2,opt,name=startedAt"`
}

// CmdStateTerminated is a terminated state of a local command.
type CmdStateTerminated struct {
	// The process id of the command.
	PID int32 `json:"pid" protobuf:"varint,1,opt,name=pid"`

	// Exit status from the last termination of the command
	ExitCode int32 `json:"exitCode" protobuf:"varint,2,opt,name=exitCode"`

	// Time at which previous execution of the command started
	StartedAt metav1.MicroTime `json:"startedAt,omitempty" protobuf:"bytes,3,opt,name=startedAt"`

	// Time at which the command last terminated
	FinishedAt metav1.MicroTime `json:"finishedAt,omitempty" protobuf:"bytes,4,opt,name=finishedAt"`

	// (brief) reason the process is terminated
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,5,opt,name=reason"`
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

// RestartOnSpec indicates the set of objects that can trigger a restart of this object.
type RestartOnSpec struct {
	// A list of file watches that can trigger a restart.
	FileWatches []string `json:"fileWatches" protobuf:"bytes,1,rep,name=fileWatches"`
}
