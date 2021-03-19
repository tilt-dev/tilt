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

// PodLogStream
//
// Streams logs from a pod on Kubernetes into the core Tilt engine.
//
// +k8s:openapi-gen=true
type PodLogStream struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PodLogStreamSpec   `json:"spec,omitempty"`
	Status PodLogStreamStatus `json:"status,omitempty"`
}

// PodLogStreamList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PodLogStreamList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []PodLogStream `json:"items"`
}

// PodLogStreamSpec defines the desired state of PodLogStream
//
// Translated into a PodLog query to the current Kubernetes cluster:
// https://pkg.go.dev/k8s.io/api/core/v1#PodLogOptions
//
// TODO(nick): Should all Kubernetes types have an object that describes
// which Kubernetes context to use?
type PodLogStreamSpec struct {
	// The name of the pod to watch. Required.
	Pod string `json:"pod,omitempty"`

	// The namespace of the pod to watch. Defaults to the kubecontext default namespace.
	//
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// An RFC3339 timestamp from which to show logs. If this value
	// precedes the time a pod was started, only logs since the pod start will be returned.
	// If this value is in the future, no logs will be returned.
	//
	// Translates directly to the underlying PodLogOptions.
	//
	// +optional
	SinceTime *metav1.Time `json:"sinceTime,omitempty"`

	// The names of containers to include in the stream.
	//
	// If `onlyContainers` and `ignoreContainers` are not set,
	// will watch all containers in the pod.
	//
	// +optional
	OnlyContainers []string `json:"onlyContainers,omitempty"`

	// The names of containers to exclude from the stream.
	//
	// If `onlyContainers` and `ignoreContainers` are not set,
	// will watch all containers in the pod.
	//
	// +optional
	IgnoreContainers []string `json:"ignoreContainers,omitempty"`
}

var _ resource.Object = &PodLogStream{}
var _ resourcestrategy.Validater = &PodLogStream{}

func (in *PodLogStream) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *PodLogStream) NamespaceScoped() bool {
	return false
}

func (in *PodLogStream) New() runtime.Object {
	return &PodLogStream{}
}

func (in *PodLogStream) NewList() runtime.Object {
	return &PodLogStreamList{}
}

func (in *PodLogStream) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "podlogstreams",
	}
}

func (in *PodLogStream) IsStorageVersion() bool {
	return true
}

func (in *PodLogStream) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &PodLogStreamList{}

func (in *PodLogStreamList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// PodLogStreamStatus defines the observed state of PodLogStream
type PodLogStreamStatus struct {
	// True when the stream is set up and streaming logs properly.
	Active bool `json:"active,omitempty"`

	// The last error message encountered while streaming.
	//
	// Empty when the stream is active and healthy.
	Error string `json:"error,omitempty"`
}

// PodLogStream implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &PodLogStream{}

func (in *PodLogStream) GetStatus() resource.StatusSubResource {
	return in.Status
}

// PodLogStreamStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &PodLogStreamStatus{}

func (in PodLogStreamStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*PodLogStream).Status = in
}
