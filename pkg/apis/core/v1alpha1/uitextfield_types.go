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

// UITextField
// +k8s:openapi-gen=true
type UITextField struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UITextFieldSpec   `json:"spec,omitempty"`
	Status UITextFieldStatus `json:"status,omitempty"`
}

// UITextFieldList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type UITextFieldList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []UITextField `json:"items"`
}

// UITextFieldSpec defines the desired state of UITextField
type UITextFieldSpec struct {
	Location UIComponentLocation `json:"location"`
	// Text to display in the field when the value is empty
	PlaceholderValue string `json:"placeholderValue"`
	// Value of the field on load
	DefaultValue string `json:"defaultValue"`
}

var _ resource.Object = &UITextField{}
var _ resourcestrategy.Validater = &UITextField{}

func (in *UITextField) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *UITextField) NamespaceScoped() bool {
	return false
}

func (in *UITextField) New() runtime.Object {
	return &UITextField{}
}

func (in *UITextField) NewList() runtime.Object {
	return &UITextFieldList{}
}

func (in *UITextField) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "uitextfields",
	}
}

func (in *UITextField) IsStorageVersion() bool {
	return true
}

func (in *UITextField) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &UITextFieldList{}

func (in *UITextFieldList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// UITextFieldStatus defines the observed state of UITextField
type UITextFieldStatus struct {
	Value string `json:"value"`
}

// UITextField implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &UITextField{}

func (in *UITextField) GetStatus() resource.StatusSubResource {
	return in.Status
}

// UITextFieldStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &UITextFieldStatus{}

func (in UITextFieldStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*UITextField).Status = in
}
