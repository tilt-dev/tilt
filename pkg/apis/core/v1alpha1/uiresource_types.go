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

// UIResource represents per-resource status data for rendering the web UI.
//
// Treat this as a legacy data structure that's more intended to make transition
// easier rather than a robust long-term API.
//
// +k8s:openapi-gen=true
type UIResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   UIResourceSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status UIResourceStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// UIResourceList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type UIResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []UIResource `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// UIResourceSpec is an empty struct.
// UIResource is a kludge for making Tilt's internal status readable, not
// for specifying behavior.
type UIResourceSpec struct {
}

var _ resource.Object = &UIResource{}
var _ resourcestrategy.Validater = &UIResource{}

func (in *UIResource) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *UIResource) NamespaceScoped() bool {
	return false
}

func (in *UIResource) New() runtime.Object {
	return &UIResource{}
}

func (in *UIResource) NewList() runtime.Object {
	return &UIResourceList{}
}

func (in *UIResource) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "uiresources",
	}
}

func (in *UIResource) IsStorageVersion() bool {
	return true
}

func (in *UIResource) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &UIResourceList{}

func (in *UIResourceList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// UIResourceStatus defines the observed state of UIResource
type UIResourceStatus struct {
}

// UIResource implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &UIResource{}

func (in *UIResource) GetStatus() resource.StatusSubResource {
	return in.Status
}

// UIResourceStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &UIResourceStatus{}

func (in UIResourceStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*UIResource).Status = in
}
