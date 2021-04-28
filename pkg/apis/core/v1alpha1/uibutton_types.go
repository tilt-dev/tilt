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

// UIButton
// +k8s:openapi-gen=true
type UIButton struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UIButtonSpec   `json:"spec,omitempty"`
	Status UIButtonStatus `json:"status,omitempty"`
}

// UIButtonList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type UIButtonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []UIButton `json:"items"`
}

// UIButtonSpec defines the desired state of UIButton
type UIButtonSpec struct {
	Location UIComponentLocation `json:"location"`

	// Text to appear on the button itself
	Text string `json:"text"`

	// The command to run when clicked.
	// Tilt will set some env vars when running the command:
	// - $BUTTON_RESOURCE_NAME
	//    If the button is on a resource, the name of the resource
	// - $TEXTFIELD_${TEXTFIELD_NAME} (if in default namespace)
	// - $TEXTFIELD_${TEXTFIELD_NAMESPACE}_${TEXTFIELD_NAME} (if not in default namespace)
	//    If the button is in the same location as a TextField named 'foo'
	//    Tilt will set $TEXTFIELD_FOO to that field's value. (name gets upper-cased)
	Cmd CmdSpec `json:"cmd"`
}

var _ resource.Object = &UIButton{}
var _ resourcestrategy.Validater = &UIButton{}

func (in *UIButton) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *UIButton) NamespaceScoped() bool {
	return false
}

func (in *UIButton) New() runtime.Object {
	return &UIButton{}
}

func (in *UIButton) NewList() runtime.Object {
	return &UIButtonList{}
}

func (in *UIButton) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "uibuttons",
	}
}

func (in *UIButton) IsStorageVersion() bool {
	return true
}

func (in *UIButton) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &UIButtonList{}

func (in *UIButtonList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// UIButtonStatus defines the observed state of UIButton
type UIButtonStatus struct {
	Cmd CmdStatus `json:"cmd"`
}

// UIButton implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &UIButton{}

func (in *UIButton) GetStatus() resource.StatusSubResource {
	return in.Status
}

// UIButtonStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &UIButtonStatus{}

func (in UIButtonStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*UIButton).Status = in
}
