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

// DockerImage
// +k8s:openapi-gen=true
type DockerImage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   DockerImageSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status DockerImageStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// DockerImageList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DockerImageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []DockerImage `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// DockerImageSpec defines the desired state of DockerImage
type DockerImageSpec struct {
}

var _ resource.Object = &DockerImage{}
var _ resourcestrategy.Validater = &DockerImage{}

func (in *DockerImage) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *DockerImage) NamespaceScoped() bool {
	return false
}

func (in *DockerImage) New() runtime.Object {
	return &DockerImage{}
}

func (in *DockerImage) NewList() runtime.Object {
	return &DockerImageList{}
}

func (in *DockerImage) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "dockerimages",
	}
}

func (in *DockerImage) IsStorageVersion() bool {
	return true
}

func (in *DockerImage) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &DockerImageList{}

func (in *DockerImageList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// DockerImageStatus defines the observed state of DockerImage
type DockerImageStatus struct {
}

// DockerImage implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &DockerImage{}

func (in *DockerImage) GetStatus() resource.StatusSubResource {
	return in.Status
}

// DockerImageStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &DockerImageStatus{}

func (in DockerImageStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*DockerImage).Status = in
}
