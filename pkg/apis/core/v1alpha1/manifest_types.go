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
	"sigs.k8s.io/apiserver-runtime/pkg/builder/resource"
	"sigs.k8s.io/apiserver-runtime/pkg/builder/resource/resourcestrategy"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Manifest
// +k8s:openapi-gen=true
type Manifest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManifestSpec   `json:"spec,omitempty"`
	Status ManifestStatus `json:"status,omitempty"`
}

// ManifestList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ManifestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []Manifest `json:"items"`
}

// ManifestSpec defines the desired state of Manifest
type ManifestSpec struct {
	Type string `json:"type"`
}

var _ resource.Object = &Manifest{}
var _ resourcestrategy.Validater = &Manifest{}

func (in *Manifest) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *Manifest) NamespaceScoped() bool {
	return false
}

func (in *Manifest) New() runtime.Object {
	return &Manifest{}
}

func (in *Manifest) NewList() runtime.Object {
	return &ManifestList{}
}

func (in *Manifest) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "core.tilt.dev",
		Version:  "v1alpha1",
		Resource: "manifests",
	}
}

func (in *Manifest) IsStorageVersion() bool {
	return true
}

func (in *Manifest) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &ManifestList{}

func (in *ManifestList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// ManifestStatus defines the observed state of Manifest
type ManifestStatus struct {
}

// Manifest implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &Manifest{}

func (in *Manifest) GetStatus() resource.StatusSubResource {
	return in.Status
}

// ManifestStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &ManifestStatus{}

func (in ManifestStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*Manifest).Status = in
}
