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

// ToggleButton
// +k8s:openapi-gen=true
type ToggleButton struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   ToggleButtonSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status ToggleButtonStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// ToggleButtonList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ToggleButtonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []ToggleButton `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ToggleButtonSpec defines the desired state of ToggleButton
type ToggleButtonSpec struct {
	// Where to display the button
	Location UIComponentLocation `json:"location" protobuf:"bytes,1,opt,name=location"`

	// Config for the button when it is "on"
	On ToggleButtonStateSpec `json:"on" protobuf:"bytes,2,opt,name=on"`

	// Config for the button when it is "off"
	Off ToggleButtonStateSpec `json:"off" protobuf:"bytes,3,opt,name=off"`

	// If `StateSource` does not point at a valid value, the initial button
	// state will be "on" or "off" depending on this bool
	DefaultOn bool `json:"defaultOn" protobuf:"varint,4,opt,name=defaultOn"`

	// Where the toggle button's state is stored
	StateSource StateSource `json:"stateSource" protobuf:"bytes,5,opt,name=stateSource"`

	// For internal Tilt use, to specify special behavior in the UI. Should typically be empty.
	// +optional
	ButtonType UIButtonType `json:"buttonType,omitempty" protobuf:"bytes,6,opt,name=buttonType,casttype=UIButtonType"`
}

// Describes a state (on/off) of a ToggleButton
type ToggleButtonStateSpec struct {
	// Text to appear on the button itself or as hover text (depending on button location).
	Text string `json:"text" protobuf:"bytes,2,opt,name=text"`

	// IconName is a Material Icon to appear next to button text or on the button itself (depending on button location).
	//
	// Valid values are icon font ligature names from the Material Icons set.
	// See https://fonts.google.com/icons for the full list of available icons.
	//
	// If both IconSVG and IconName are specified, IconSVG will take precedence.
	//
	// +optional
	IconName string `json:"iconName,omitempty" protobuf:"bytes,3,opt,name=iconName"`

	// IconSVG is an SVG to use as the icon to appear next to button text or on the button itself (depending on button
	// location).
	//
	// This should be an <svg> element scaled for a 24x24 viewport.
	//
	// If both IconSVG and IconName are specified, IconSVG will take precedence.
	//
	// +optional
	IconSVG string `json:"iconSVG,omitempty" protobuf:"bytes,4,opt,name=iconSVG"`

	// If true, clicking the button in this state requires a second click
	// to confirm.
	//
	// +optional
	RequiresConfirmation bool `json:"requiresConfirmation,omitempty" protobuf:"varint,5,opt,name=requiresConfirmation"`
}

// Describes where a ToggleButton's state is stored.
// Exactly one type of source must be set.
type StateSource struct {
	// State is stored in a ConfigMap.
	//
	// +optional
	ConfigMap *ConfigMapStateSource `json:"configMap,omitempty" protobuf:"bytes,1,opt,name=configMap"`
}

// Describes how a ToggleButton's state is stored in a ConfigMap.
// The ConfigMap must be created separately - the ToggleButton will not automatically create it.
type ConfigMapStateSource struct {
	// Name of the ConfigMap
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`

	// Key within the ConfigMap
	Key string `json:"key" protobuf:"bytes,2,opt,name=key"`

	// ConfigMap value corresponding to the button's "on" state.
	// If not specified, "true" will be used.
	// +optional
	OnValue string `json:"onValue,omitempty" protobuf:"bytes,3,opt,name=onValue"`

	// ConfigMap value corresponding to the button's "off" state
	// If not specified, "false" will be used.
	OffValue string `json:"offValue,omitempty" protobuf:"bytes,4,opt,name=offValue"`
}

var _ resource.Object = &ToggleButton{}
var _ resourcestrategy.Validater = &ToggleButton{}

func (in *ToggleButton) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *ToggleButton) GetSpec() interface{} {
	return &in.Spec
}

func (in *ToggleButton) NamespaceScoped() bool {
	return false
}

func (in *ToggleButton) New() runtime.Object {
	return &ToggleButton{}
}

func (in *ToggleButton) NewList() runtime.Object {
	return &ToggleButtonList{}
}

func (in *ToggleButton) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "togglebuttons",
	}
}

func (in *ToggleButton) IsStorageVersion() bool {
	return true
}

func (in *ToggleButton) Validate(ctx context.Context) field.ErrorList {
	var result field.ErrorList
	if in.Spec.StateSource.ConfigMap == nil {
		result = append(result, field.Invalid(field.NewPath("Spec", "StateSource"), in.Spec.StateSource, "must specify exactly one kind of StateSource"))
	} else {
		if in.Spec.StateSource.ConfigMap.OffValue == in.Spec.StateSource.ConfigMap.OnValue {
			result = append(result, field.Invalid(field.NewPath("Spec", "StateSource", "ConfigMap"), in.Spec.StateSource.ConfigMap, "OnValue and OffValue must differ"))
		}
	}
	return result
}

var _ resource.ObjectList = &ToggleButtonList{}

func (in *ToggleButtonList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// ToggleButtonStatus defines the observed state of ToggleButton
type ToggleButtonStatus struct {
	// If healthy, empty. If non-healthy, specifies a problem the ToggleButton encountered
	// +optional
	Error string `json:"error" protobuf:"bytes,1,opt,name=error"`
}

// ToggleButton implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &ToggleButton{}

func (in *ToggleButton) GetStatus() resource.StatusSubResource {
	return in.Status
}

// ToggleButtonStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &ToggleButtonStatus{}

func (in ToggleButtonStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*ToggleButton).Status = in
}
