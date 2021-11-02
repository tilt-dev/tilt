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
	"strings"

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
// +tilt:starlark-gen=true
type UIButton struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   UIButtonSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status UIButtonStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// UIButtonList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type UIButtonList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []UIButton `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// UIButtonSpec defines the desired state of UIButton
type UIButtonSpec struct {
	// Location associates the button with another component for layout.
	Location UIComponentLocation `json:"location" protobuf:"bytes,1,opt,name=location"`

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

	// If true, the button will be rendered, but with an effect indicating it's
	// disabled. It will also be unclickable.
	//
	// +optional
	Disabled bool `json:"disabled,omitempty" protobuf:"varint,5,opt,name=disabled"`

	// If true, the UI will require the user to click the button a second time to
	// confirm before taking action
	//
	// +optional
	RequiresConfirmation bool `json:"requiresConfirmation,omitempty" protobuf:"varint,7,opt,name=requiresConfirmation"`

	// Any inputs for this button.
	// +optional
	Inputs []UIInputSpec `json:"inputs,omitempty" protobuf:"bytes,6,rep,name=inputs"`

	// For internal Tilt use, to specify special behavior in the UI. Should typically be empty.
	// +optional
	ButtonType UIButtonType `json:"buttonType,omitempty" protobuf:"bytes,8,opt,name=buttonType,casttype=UIButtonType"`
}

type UIButtonType string

const (
	UIButtonTypeDisableToggle UIButtonType = "DisableToggle"
)

// UIComponentLocation specifies where to put a UI component.
type UIComponentLocation struct {
	// ComponentID is the identifier of the parent component to associate this component with.
	//
	// For example, this is a resource name if the ComponentType is Resource.
	ComponentID string `json:"componentID" protobuf:"bytes,1,opt,name=componentID"`
	// ComponentType is the type of the parent component.
	ComponentType ComponentType `json:"componentType" protobuf:"bytes,2,opt,name=componentType,casttype=ComponentType"`
}

type ComponentType string

const (
	ComponentTypeResource ComponentType = "Resource"
	ComponentTypeGlobal   ComponentType = "Global"
)

type UIComponentLocationResource struct {
	ResourceName string `json:"resourceName" protobuf:"bytes,1,opt,name=resourceName"`
}

var _ resource.Object = &UIButton{}
var _ resourcestrategy.Validater = &UIButton{}

func (in *UIButton) GetSpec() interface{} {
	return in.Spec
}

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
	var fieldErrors field.ErrorList

	if in.Spec.Text == "" {
		fieldErrors = append(fieldErrors, field.Required(
			field.NewPath("spec.text"), "Button text cannot be empty"))
	}

	locField := field.NewPath("spec.location")
	if in.Spec.Location.ComponentID == "" {
		fieldErrors = append(fieldErrors, field.Required(
			locField.Child("componentID"), "Parent component ID is required"))
	}
	if in.Spec.Location.ComponentType == "" {
		fieldErrors = append(fieldErrors, field.Required(
			locField.Child("componentType"), "Parent component type is required"))
	}

	if in.Spec.IconSVG != "" {
		// do a basic sanity check to catch things like users passing a filename or a <path> directly
		if !strings.Contains(in.Spec.IconSVG, "<svg") {
			fieldErrors = append(fieldErrors, field.Invalid(field.NewPath("spec.iconSVG"), in.Spec.IconSVG,
				"Invalid <svg> element"))
		}
	}

	seenInputIDs := make(map[string]bool)
	for i, input := range in.Spec.Inputs {
		if seenInputIDs[input.Name] {
			fieldErrors = append(fieldErrors, field.Duplicate(field.NewPath("spec").Child("inputs").Index(i).Child("id"), input))
		}
		seenInputIDs[input.Name] = true
		fieldErrors = append(fieldErrors, input.Validate(ctx, field.NewPath("spec"))...)
	}

	return fieldErrors
}

var _ resource.ObjectList = &UIButtonList{}

func (in *UIButtonList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// Describes a text input field attached to a button.
type UITextInputSpec struct {
	// Initial value for this field.
	//
	// +optional
	DefaultValue string `json:"defaultValue,omitempty" protobuf:"bytes,1,opt,name=defaultValue"`

	// A short hint that describes the expected input of this field.
	//
	// +optional
	Placeholder string `json:"placeholder,omitempty" protobuf:"bytes,2,opt,name=placeholder"`
}

type UITextInputStatus struct {
	// The content of the text input.
	Value string `json:"value" protobuf:"bytes,1,opt,name=value"`
}

// Describes a boolean checkbox input field attached to a button.
type UIBoolInputSpec struct {
	// Whether the input is initially true or false.
	// +optional
	DefaultValue bool `json:"defaultValue,omitempty" protobuf:"varint,1,opt,name=defaultValue"`

	// If the input's value is converted to a string, use this when the value is true.
	// If unspecified, its string value will be `"true"`
	// +optional
	TrueString *string `json:"trueString,omitempty" protobuf:"bytes,2,opt,name=trueString"`

	// If the input's value is converted to a string, use this when the value is false.
	// If unspecified, its string value will be `"false"`
	// +optional
	FalseString *string `json:"falseString,omitempty" protobuf:"bytes,3,opt,name=falseString"`
}

type UIBoolInputStatus struct {
	Value bool `json:"value" protobuf:"varint,1,opt,name=value"`
}

// Describes a hidden input field attached to a button,
// with a value to pass on any submit.
type UIHiddenInputSpec struct {
	Value string `json:"value" protobuf:"bytes,1,opt,name=value"`
}

type UIHiddenInputStatus struct {
	Value string `json:"value" protobuf:"bytes,1,opt,name=value"`
}

// Defines an Input to render in the UI.
// If UIButton is analogous to an HTML <form>,
// UIInput is analogous to an HTML <input>.
type UIInputSpec struct {
	// Name of this input. Must be unique within the UIButton.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`

	// A label to display next to this input in the UI.
	// +optional
	Label string `json:"label" protobuf:"bytes,2,opt,name=label"`

	// Exactly one of the following must be non-nil.

	// A Text input that takes a string.
	// +optional
	Text *UITextInputSpec `json:"text,omitempty" protobuf:"bytes,3,opt,name=text"`

	// A Bool input that is true or false
	// +optional
	Bool *UIBoolInputSpec `json:"bool,omitempty" protobuf:"bytes,4,opt,name=bool"`

	// An input that has a constant value and does not display to the user
	// +optional
	Hidden *UIHiddenInputSpec `json:"hidden,omitempty" protobuf:"bytes,5,opt,name=hidden"`
}

func (in *UIInputSpec) Validate(_ context.Context, path *field.Path) field.ErrorList {
	var fieldErrors field.ErrorList

	numInputTypes := 0
	if in.Text != nil {
		numInputTypes += 1
	}
	if in.Bool != nil {
		numInputTypes += 1
	}
	if in.Hidden != nil {
		numInputTypes += 1
	}

	if numInputTypes != 1 {
		fieldErrors = append(fieldErrors, field.Invalid(path, in, "must specify exactly one input type"))
	}

	return fieldErrors
}

// The status corresponding to a UIInputSpec
type UIInputStatus struct {
	// Name of the input whose status this is. Must match the `Name` of a corresponding
	// UIInputSpec.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`

	// The same one of these should be non-nil as on the corresponding UITextInputSpec

	// The status of the input, if it's text
	// +optional
	Text *UITextInputStatus `json:"text,omitempty" protobuf:"bytes,2,opt,name=text"`

	// The status of the input, if it's a bool
	// +optional
	Bool *UIBoolInputStatus `json:"bool,omitempty" protobuf:"bytes,3,opt,name=bool"`

	// The status of the input, if it's a hidden
	// +optional
	Hidden *UIHiddenInputStatus `json:"hidden,omitempty" protobuf:"bytes,4,opt,name=hidden"`
}

// UIButtonStatus defines the observed state of UIButton
type UIButtonStatus struct {
	// LastClickedAt is the timestamp of the last time the button was clicked.
	//
	// If the button has never clicked before, this will be the zero-value/null.
	LastClickedAt metav1.MicroTime `json:"lastClickedAt,omitempty" protobuf:"bytes,1,opt,name=lastClickedAt"`

	// Status of any inputs on this button.
	// +optional
	Inputs []UIInputStatus `json:"inputs,omitempty" protobuf:"bytes,2,rep,name=inputs"`
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
