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

// ImageMap expresses the mapping from an image reference to a real, pushed
// image in an image registry that a container runtime can access.
//
// Another way to think about the ImageMap is that ImageMapSpec is a mutable
// image reference (where the image might not exist yet), but ImageMapStatus is
// an immutable image reference (where, if an image is specified, it always
// exists).
//
// ImageMap does not specify how the image is built or who is responsible for
// building this. But any API that builds images should produce an ImageMap.
//
// For example, a builder that builds to a local image registry might create
// a map from: 'my-apiserver:dev' to 'localhost:5000/my-apiserver:content-based-label'.
//
// +k8s:openapi-gen=true
type ImageMap struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   ImageMapSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status ImageMapStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// ImageMapList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ImageMapList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []ImageMap `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ImageMapSpec defines the desired state of ImageMap
type ImageMapSpec struct {
}

var _ resource.Object = &ImageMap{}
var _ resourcestrategy.Validater = &ImageMap{}

func (in *ImageMap) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *ImageMap) NamespaceScoped() bool {
	return false
}

func (in *ImageMap) New() runtime.Object {
	return &ImageMap{}
}

func (in *ImageMap) NewList() runtime.Object {
	return &ImageMapList{}
}

func (in *ImageMap) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "imagemaps",
	}
}

func (in *ImageMap) IsStorageVersion() bool {
	return true
}

func (in *ImageMap) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &ImageMapList{}

func (in *ImageMapList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// ImageMapStatus defines the observed state of ImageMap
type ImageMapStatus struct {
}

// ImageMap implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &ImageMap{}

func (in *ImageMap) GetStatus() resource.StatusSubResource {
	return in.Status
}

// ImageMapStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &ImageMapStatus{}

func (in ImageMapStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*ImageMap).Status = in
}
