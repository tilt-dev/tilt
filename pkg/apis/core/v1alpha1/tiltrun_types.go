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
	// StartTime is when the Tilt engine was first started.
	StartTime metav1.MicroTime `json:"startTime"`
	// Targets are normalized representations of the servers/jobs managed by this TiltRun.
	//
	// A resource from a Tiltfile might produce one or more targets. A target can also be shared across
	// multiple resources (e.g. an image referenced by multiple K8s pods).
	Targets []Target `json:"resources"`

	// Done indicates whether this TiltRun has completed its work and is ready to exit.
	Done bool `json:"done"`
	// Error is a non-empty string when the TiltRun is Done but encountered a failure as defined by the ExitCondition
	// from the TiltRunSpec.
	Error string `json:"error,omitempty"`
}

// Target is a server or job whose execution is managed as part of this TiltRun.
type Target struct {
	// Name is the name of the target; this is auto-generated from Tiltfile resources.
	Name string `json:"name"`
	// Type is the execution profile for this resource.
	//
	// Job targets run to completion (e.g. a build script or database migration script).
	// Server targets run indefinitely (e.g. an HTTP server).
	Type TargetType `json:"type"`
	// Resources are one or more Tiltfile resources that this target is associated with.
	Resources []string `json:"resources,omitempty"`
	// State provides information about the current status of the target.
	State TargetState `json:"runtime,omitempty"`
}

// TargetType describes a high-level categorization about the expected execution behavior for the target.
type TargetType string

const (
	// TargetTypeJob is a target that is expected to run to completion.
	TargetTypeJob TargetType = "job"
	// TargetTypeServer is a target that runs indefinitely.
	TargetTypeServer TargetType = "server"
)

// TargetState describes the current execution status for a target.
//
// Either EXACTLY one of Waiting, Active, or Terminated will be populated or NONE of them will be.
// In the event that all states are null, the target is currently inactive or disabled and should not
// be expected to execute.
type TargetState struct {
	// Waiting being non-nil indicates that the next execution of the target has been queued but not yet started.
	Waiting *TargetStateWaiting `json:"pending,omitempty"`
	// Active being non-nil indicates that the target is currently executing.
	Active *TargetStateActive `json:"active,omitempty"`
	// Terminated being non-nil indicates that the target finished execution either normally or due to failure.
	Terminated *TargetStateTerminated `json:"terminated,omitempty"`
}

// TargetStateWaiting is a target that has been enqueued for execution but has not yet started.
type TargetStateWaiting struct {
	// Reason is a description for why the target is waiting and not yet active.
	Reason string `json:"reason"`
}

// TargetStateActive is a target that is currently running but has not yet finished.
type TargetStateActive struct {
	// StartTime is when execution began.
	StartTime metav1.MicroTime `json:"startTime"`
	// Ready indicates that the target has passed readiness checks.
	//
	// If the target does not use readiness checks, this is always true.
	Ready bool
}

// TargetStateTerminated is a target that finished running, either because it completed successfully or
// encountered an error.
type TargetStateTerminated struct {
	// StartTime is when the target began executing.
	StartTime metav1.MicroTime `json:"startTime"`
	// FinishTime is when the target stopped executing.
	FinishTime metav1.MicroTime `json:"finishTime"`
	// Error is a non-empty string if the target encountered a failure during execution that caused it to stop.
	//
	// For targets of type TargetTypeServer, this is always populated, as the target is expected to run indefinitely.
	Error string `json:"error,omitempty"`
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
