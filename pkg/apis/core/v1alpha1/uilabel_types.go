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

// UILabel
// +k8s:openapi-gen=true
type UILabel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UILabelSpec   `json:"spec,omitempty"`
	Status UILabelStatus `json:"status,omitempty"`
}

// UILabelList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type UILabelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []UILabel `json:"items"`
}

// UILabelSpec defines the desired state of UILabel
type UILabelSpec struct {
}

var _ resource.Object = &UILabel{}
var _ resourcestrategy.Validater = &UILabel{}

func (in *UILabel) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *UILabel) NamespaceScoped() bool {
	return false
}

func (in *UILabel) New() runtime.Object {
	return &UILabel{}
}

func (in *UILabel) NewList() runtime.Object {
	return &UILabelList{}
}

func (in *UILabel) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "uilabels",
	}
}

func (in *UILabel) IsStorageVersion() bool {
	return true
}

func (in *UILabel) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &UILabelList{}

func (in *UILabelList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// UILabelStatus defines the observed state of UILabel
type UILabelStatus struct {
}

// UILabel implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &UILabel{}

func (in *UILabel) GetStatus() resource.StatusSubResource {
	return in.Status
}

// UILabelStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &UILabelStatus{}

func (in UILabelStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*UILabel).Status = in
}
