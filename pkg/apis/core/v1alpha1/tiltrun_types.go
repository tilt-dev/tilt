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
	"strings"

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

// TiltRun provides introspective data about the status of the Tilt process.
// +k8s:openapi-gen=true
type TiltRun struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TiltRunSpec   `json:"spec,omitempty"`
	Status TiltRunStatus `json:"status,omitempty"`
}

// TiltRunList is a list of TiltRun objects.
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TiltRunList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []TiltRun `json:"items"`
}

// TiltRunSpec defines the desired state of TiltRun
type TiltRunSpec struct {
	// TiltfilePath is the path to the Tiltfile for the run. It cannot be empty.
	TiltfilePath string `json:"tiltfilePath"`
	// ExitCondition defines the criteria for Tilt to exit.
	ExitCondition ExitCondition `json:"exitCondition"`
}

type ExitCondition string

const (
	// ExitConditionManual cedes control to the user and will not exit based on resource status.
	//
	// This is used by `tilt up`.
	ExitConditionManual ExitCondition = "manual"
	// ExitConditionCI terminates upon the first encountered build or runtime failure or after all resources have been
	// started successfully.
	//
	// This is used by `tilt ci`.
	ExitConditionCI ExitCondition = "ci"
)

var exitConditions = []ExitCondition{ExitConditionManual, ExitConditionCI}

var _ resource.Object = &TiltRun{}
var _ resourcestrategy.Validater = &TiltRun{}

func (in *TiltRun) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *TiltRun) NamespaceScoped() bool {
	return false
}

func (in *TiltRun) New() runtime.Object {
	return &TiltRun{}
}

func (in *TiltRun) NewList() runtime.Object {
	return &TiltRunList{}
}

func (in *TiltRun) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "tiltruns",
	}
}

func (in *TiltRun) IsStorageVersion() bool {
	return true
}

func (in *TiltRun) Validate(_ context.Context) field.ErrorList {
	var fieldErrors field.ErrorList
	if in.Spec.TiltfilePath == "" {
		fieldErrors = append(fieldErrors, field.Required(field.NewPath("tiltfilePath"), "cannot be empty"))
	}
	validExitCondition := false
	for _, v := range exitConditions {
		if v == in.Spec.ExitCondition {
			validExitCondition = true
			break
		}
	}
	if !validExitCondition {
		var detailMsg strings.Builder
		detailMsg.WriteString("valid values: ")
		for i, ec := range exitConditions {
			if i != 0 {
				detailMsg.WriteString(", ")
			}
			detailMsg.WriteString(string(ec))
		}

		fieldErrors = append(fieldErrors, field.Invalid(
			field.NewPath("exitCondition"),
			in.Spec.ExitCondition,
			detailMsg.String()))
	}
	return fieldErrors
}

var _ resource.ObjectList = &TiltRunList{}

func (in *TiltRunList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// TiltRunStatus defines the observed state of TiltRun
type TiltRunStatus struct {
	// PID is the process identifier for this instance of Tilt.
	PID int64 `json:"pid"`
	// StartTime is when the Tilt engine was first started and began processing resources.
	StartTime metav1.MicroTime `json:"startTime"`
	// Resources are normalized state representations of the servers/jobs managed by this TiltRun.
	Resources []ResourceState `json:"resources"`

	// Done indicates whether this TiltRun has completed its work and is ready to exit.
	Done bool `json:"done"`
	// Error is a non-empty string when the TiltRun is Done but encountered a failure as defined by the ExitCondition
	// from the TiltRunSpec.
	Error string `json:"error,omitempty"`
}

// ResourceState contains a normalized representation of build and runtime state for a resource managed by this TiltRun.
type ResourceState struct {
	// Name is the name of the resource, typically defined via a call to a resource function in the Tiltfile.
	Name string `json:"name"`
	// Build provides information about pending/active/terminated build(s) for the resource.
	//
	// If nil, the resource does not perform builds (for example, a local resource without a serve_cmd).
	Build *BuildState `json:"build,omitempty"`
	// Runtime provides information about the current execution of the resource.
	Runtime RuntimeState `json:"runtime,omitempty"`
}

// BuildState includes details about a currently pending build, currently active build, and (last) terminated build.
type BuildState struct {
	// Pending gives details about the currently enqueued build (if any).
	Pending *BuildStatePending `json:"pending,omitempty"`
	// Active gives details about the currently running build (if any).
	Active *BuildStateActive `json:"active,omitempty"`
	// Terminated gives details about the last finished build (if any).
	Terminated *BuildStateTerminated `json:"terminated,omitempty"`
}

// BuildStatePending is a build that has been enqueued for execution but has not yet started.
type BuildStatePending struct {
	// TriggerTime is when the earliest event occurred (e.g. file change) occurred that resulted in a build being
	// enqueued.
	TriggerTime metav1.MicroTime `json:"triggerTime"`
	// Reason is a description for why the build is being triggered. There may be more than one cause, but only a
	// single reason is provided.
	Reason string `json:"reason"`
}

// BuildStateActive is a build that is currently running but has not yet finished.
type BuildStateActive struct {
	// StartTime is when the build began.
	StartTime metav1.MicroTime `json:"startTime"`
}

// BuildStateTerminated is a build that finished running, either because it completed successfully or encountered an error.
type BuildStateTerminated struct {
	// StartTime is when the build began.
	StartTime metav1.MicroTime `json:"startTime"`
	// FinishTime is when the build stopped.
	FinishTime metav1.MicroTime `json:"finishTime"`
	// Error is a non-empty string if the build did not complete successfully.
	Error string `json:"error,omitempty"`
}

// RuntimeStatus describes the current state of resource execution. Some values might not be used by every RuntimeType.
type RuntimeStatus string

const (
	// RuntimeStatusPending indicates that the resource has not yet been started because it is waiting on another
	// resource.
	RuntimeStatusPending RuntimeStatus = "pending"
	// RuntimeStatusRunning indicates that the resource is currently executing.
	RuntimeStatusRunning RuntimeStatus = "running"
	// RuntimeStatusSucceeded indicates that the resource ran to completion successfully.
	//
	// This is only used for a RuntimeType of "job"; "server" resources never exit during normal operation.
	RuntimeStatusSucceeded RuntimeStatus = "succeeded"
	// RuntimeStatusFailed indicates that the resource encountered an error during execution.
	RuntimeStatusFailed RuntimeStatus = "failed"
	// RuntimeStatusDisabled indicates that the resource has been requested to not execute at this time.
	RuntimeStatusDisabled RuntimeStatus = "disabled"
	// RuntimeStatusUnknown indicates that the status is not currently known; this can occur during initialization.
	RuntimeStatusUnknown RuntimeStatus = "unknown"
)

// RuntimeType describes a high-level categorization about the expected behavior for the resource.
//
// This can be used in conjunction with RuntimeStatus to fully understand the resource's runtime state.
type RuntimeType string

const (
	// RuntimeTypeJob is a resource that is expected to run to completion.
	RuntimeTypeJob RuntimeType = "job"
	// RuntimeTypeServer is a resource that runs indefinitely.
	RuntimeTypeServer RuntimeType = "server"
)

// RuntimeState describes the current execution state for a resource.
type RuntimeState struct {
	// Type is the execution profile for this resource to be used in conjunction with Status.
	Type RuntimeType `json:"type"`
	// Status is the current execution status for this resource.
	Status RuntimeStatus `json:"status"`
	// Error is a non-empty string describing the failure if Status is "failed".
	Error string `json:"error,omitempty"`

	// TODO(milas): this should probably have *StartTime/*EndTime but we don't have that available right now
}

// TiltRun implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &TiltRun{}

func (in *TiltRun) GetStatus() resource.StatusSubResource {
	return in.Status
}

// TiltRunStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &TiltRunStatus{}

func (in TiltRunStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*TiltRun).Status = in
}
