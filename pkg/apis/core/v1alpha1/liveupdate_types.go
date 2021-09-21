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

// LiveUpdate
// +k8s:openapi-gen=true
type LiveUpdate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   LiveUpdateSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status LiveUpdateStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// LiveUpdateList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type LiveUpdateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []LiveUpdate `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// LiveUpdateSpec defines the desired state of LiveUpdate
type LiveUpdateSpec struct {
}

var _ resource.Object = &LiveUpdate{}
var _ resourcestrategy.Validater = &LiveUpdate{}

func (in *LiveUpdate) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *LiveUpdate) NamespaceScoped() bool {
	return false
}

func (in *LiveUpdate) New() runtime.Object {
	return &LiveUpdate{}
}

func (in *LiveUpdate) NewList() runtime.Object {
	return &LiveUpdateList{}
}

func (in *LiveUpdate) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "liveupdates",
	}
}

func (in *LiveUpdate) IsStorageVersion() bool {
	return true
}

func (in *LiveUpdate) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &LiveUpdateList{}

func (in *LiveUpdateList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// LiveUpdateStatus defines the observed state of LiveUpdate
type LiveUpdateStatus struct {
}

// LiveUpdate implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &LiveUpdate{}

func (in *LiveUpdate) GetStatus() resource.StatusSubResource {
	return in.Status
}

// LiveUpdateStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &LiveUpdateStatus{}

func (in LiveUpdateStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*LiveUpdate).Status = in
}
