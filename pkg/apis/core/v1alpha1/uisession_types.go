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

// UISession represents global status data for rendering the web UI.
//
// Treat this as a legacy data structure that's more intended to make transition
// easier rather than a robust long-term API.
//
// Per-resource status data should be stored in UIResource.
//
// +k8s:openapi-gen=true
type UISession struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   UISessionSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status UISessionStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// UISessionList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type UISessionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []UISession `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// UISessionSpec is an empty struct.
// UISession is a kludge for making Tilt's internal status readable, not
// for specifying behavior.
type UISessionSpec struct {
}

var _ resource.Object = &UISession{}
var _ resourcestrategy.Validater = &UISession{}

func (in *UISession) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *UISession) NamespaceScoped() bool {
	return false
}

func (in *UISession) New() runtime.Object {
	return &UISession{}
}

func (in *UISession) NewList() runtime.Object {
	return &UISessionList{}
}

func (in *UISession) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "uisessions",
	}
}

func (in *UISession) IsStorageVersion() bool {
	return true
}

func (in *UISession) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &UISessionList{}

func (in *UISessionList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// UISessionStatus defines the observed state of UISession
type UISessionStatus struct {
}

// UISession implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &UISession{}

func (in *UISession) GetStatus() resource.StatusSubResource {
	return in.Status
}

// UISessionStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &UISessionStatus{}

func (in UISessionStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*UISession).Status = in
}
