/*
Copyright 2021 The Tilt Dev Authors

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
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource/resourcerest"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource/resourcestrategy"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DockerComposeLogStream
// +k8s:openapi-gen=true
type DockerComposeLogStream struct {
	metav1.TypeMeta   `json:",inline" tstype:"-"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   DockerComposeLogStreamSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status DockerComposeLogStreamStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// DockerComposeLogStreamList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DockerComposeLogStreamList struct {
	metav1.TypeMeta `json:",inline" tstype:"-"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []DockerComposeLogStream `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// DockerComposeLogStreamSpec defines the desired state of DockerComposeLogStream
type DockerComposeLogStreamSpec struct {
	// The name of the service to stream from.
	Service string `json:"service" protobuf:"bytes,1,opt,name=service"`

	// A specification of the project the service belongs to.
	//
	// Each service spec keeps its own copy of the project spec.
	Project DockerComposeProject `json:"project" protobuf:"bytes,2,opt,name=project"`
}

var _ resource.Object = &DockerComposeLogStream{}
var _ resourcerest.SingularNameProvider = &DockerComposeLogStream{}
var _ resourcestrategy.Validater = &DockerComposeLogStream{}

func (in *DockerComposeLogStream) GetSingularName() string {
	return "dockercomposelogstream"
}

func (in *DockerComposeLogStream) ShortNames() []string {
	return []string{"dclog", "dcls"}
}

func (in *DockerComposeLogStream) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *DockerComposeLogStream) NamespaceScoped() bool {
	return false
}

func (in *DockerComposeLogStream) New() runtime.Object {
	return &DockerComposeLogStream{}
}

func (in *DockerComposeLogStream) NewList() runtime.Object {
	return &DockerComposeLogStreamList{}
}

func (in *DockerComposeLogStream) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "dockercomposelogstreams",
	}
}

func (in *DockerComposeLogStream) IsStorageVersion() bool {
	return true
}

func (in *DockerComposeLogStream) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &DockerComposeLogStreamList{}

func (in *DockerComposeLogStreamList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// DockerComposeLogStreamStatus defines the observed state of DockerComposeLogStream
type DockerComposeLogStreamStatus struct {
	// When we last started the log streamer.
	StartedAt metav1.MicroTime `json:"startedAt,omitempty" protobuf:"bytes,1,opt,name=startedAt"`

	// Contains an error message when the log streamer is in an error state.
	Error string `json:"error,omitempty" protobuf:"bytes,2,opt,name=error"`
}

// DockerComposeLogStream implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &DockerComposeLogStream{}

func (in *DockerComposeLogStream) GetStatus() resource.StatusSubResource {
	return in.Status
}

// DockerComposeLogStreamStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &DockerComposeLogStreamStatus{}

func (in DockerComposeLogStreamStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*DockerComposeLogStream).Status = in
}
