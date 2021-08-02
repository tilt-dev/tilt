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

// GlobalExtension defines an extension that's evaluated on Tilt startup.
// +k8s:openapi-gen=true
type GlobalExtension struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   GlobalExtensionSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status GlobalExtensionStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// GlobalExtensionList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type GlobalExtensionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []GlobalExtension `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// GlobalExtensionSpec defines the desired state of GlobalExtension
type GlobalExtensionSpec struct {
	// RepoName specifies the ExtensionRepo object where we should find this extension.
	//
	// The GlobalExtension controller should watch for changes to this repo, and
	// may update if this repo is deleted or moved.
	RepoName string `json:"repoName" protobuf:"bytes,1,opt,name=repoName"`
}

var _ resource.Object = &GlobalExtension{}
var _ resourcestrategy.Validater = &GlobalExtension{}

func (in *GlobalExtension) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *GlobalExtension) NamespaceScoped() bool {
	return false
}

func (in *GlobalExtension) New() runtime.Object {
	return &GlobalExtension{}
}

func (in *GlobalExtension) NewList() runtime.Object {
	return &GlobalExtensionList{}
}

func (in *GlobalExtension) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "globalextensions",
	}
}

func (in *GlobalExtension) IsStorageVersion() bool {
	return true
}

func (in *GlobalExtension) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &GlobalExtensionList{}

func (in *GlobalExtensionList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// GlobalExtensionStatus defines the observed state of GlobalExtension
type GlobalExtensionStatus struct {
	// Contains information about any problems loading the extension.
	Error string `json:"error,omitempty" protobuf:"bytes,1,opt,name=error"`

	// The path to the extension on disk. This location should be shared
	// and readable by all Tilt instances.
	Path string `json:"path,omitempty" protobuf:"bytes,2,opt,name=path"`
}

// GlobalExtension implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &GlobalExtension{}

func (in *GlobalExtension) GetStatus() resource.StatusSubResource {
	return in.Status
}

// GlobalExtensionStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &GlobalExtensionStatus{}

func (in GlobalExtensionStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*GlobalExtension).Status = in
}
