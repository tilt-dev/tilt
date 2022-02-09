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

// Tiltfile is the main way users add services to Tilt.
//
// The Tiltfile evaluator executes the Tiltfile, then adds all the objects
// it creates as children of the Tiltfile object.
//
// +k8s:openapi-gen=true
type Tiltfile struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   TiltfileSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status TiltfileStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// TiltfileList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type TiltfileList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []Tiltfile `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// TiltfileSpec defines the desired state of Tiltfile
type TiltfileSpec struct {
	// The path to the Tiltfile on disk.
	Path string `json:"path" protobuf:"bytes,1,opt,name=path"`

	// A set of labels to apply to all objects owned by this Tiltfile.
	// +optional
	Labels map[string]string `json:"labels,omitempty" protobuf:"bytes,2,rep,name=labels"`

	// Objects that can trigger a re-execution of this Tiltfile.
	// +optional
	RestartOn *RestartOnSpec `json:"restartOn,omitempty" protobuf:"bytes,3,opt,name=restartOn"`

	// Arguments to the Tiltfile.
	//
	// Arguments can be positional (['a', 'b', 'c']) or flag-based ('--to-edit=a').
	// By default, a list of arguments indicates the list of services in the tiltfile
	// that should be enabled.
	//
	// +optional
	Args []string `json:"args,omitempty" protobuf:"bytes,4,rep,name=args"`

	// Objects that can trigger the cancellation of an execution of this Tiltfile.
	// +optional
	CancelOn *CancelOnSpec `json:"cancelOn,omitempty"`
}

var _ resource.Object = &Tiltfile{}
var _ resourcestrategy.Validater = &Tiltfile{}

func (in *Tiltfile) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *Tiltfile) NamespaceScoped() bool {
	return false
}

func (in *Tiltfile) New() runtime.Object {
	return &Tiltfile{}
}

func (in *Tiltfile) NewList() runtime.Object {
	return &TiltfileList{}
}

func (in *Tiltfile) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "tiltfiles",
	}
}

func (in *Tiltfile) IsStorageVersion() bool {
	return true
}

func (in *Tiltfile) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &TiltfileList{}

func (in *TiltfileList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// TiltfileStatus defines the observed state of Tiltfile
type TiltfileStatus struct {
	// Details about a waiting tiltfile.
	// +optional
	Waiting *TiltfileStateWaiting `json:"waiting,omitempty" protobuf:"bytes,1,opt,name=waiting"`

	// Details about a running tiltfile.
	// +optional
	Running *TiltfileStateRunning `json:"running,omitempty" protobuf:"bytes,2,opt,name=running"`

	// Details about a terminated tiltfile.
	// +optional
	Terminated *TiltfileStateTerminated `json:"terminated,omitempty" protobuf:"bytes,3,opt,name=terminated"`
}

// Tiltfile implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &Tiltfile{}

func (in *Tiltfile) GetStatus() resource.StatusSubResource {
	return in.Status
}

// TiltfileStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &TiltfileStatus{}

func (in TiltfileStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*Tiltfile).Status = in
}

// TiltfileStateWaiting is a waiting state of a tiltfile execution.
type TiltfileStateWaiting struct {
	// (brief) reason the tiltfile is waiting.
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,1,opt,name=reason"`
}

// TiltfileStateRunning is a running state of a tiltfile execution.
type TiltfileStateRunning struct {
	// The reason why this tiltfile was built.
	// May contain more than one reason.
	// +optional
	Reasons []string `json:"reasons,omitempty" protobuf:"bytes,1,rep,name=reasons"`

	// Time at which previous execution of the command started.
	StartedAt metav1.MicroTime `json:"startedAt,omitempty" protobuf:"bytes,2,opt,name=startedAt"`
}

// TiltfileStateTerminated is a terminated state of a tiltfile execution.
type TiltfileStateTerminated struct {
	// The reasons why this tiltfile was built.
	// May contain more than one reason.
	// +optional
	Reasons []string `json:"reasons,omitempty" protobuf:"bytes,1,rep,name=reasons"`

	// Error message if this tiltfile execution failed.
	// +optional
	Error string `json:"error,omitempty" protobuf:"bytes,2,opt,name=error"`

	// Time at which previous execution of the command started.
	StartedAt metav1.MicroTime `json:"startedAt,omitempty" protobuf:"bytes,3,opt,name=startedAt"`

	// Time at which the command last terminated.
	FinishedAt metav1.MicroTime `json:"finishedAt,omitempty" protobuf:"bytes,4,opt,name=finishedAt"`

	// Number of warnings generated by this Tiltfile.
	// (brief) reason the process is terminated
	// +optional
	WarningCount int32 `json:"warningCount,omitempty" protobuf:"varint,5,opt,name=warningCount"`
}
