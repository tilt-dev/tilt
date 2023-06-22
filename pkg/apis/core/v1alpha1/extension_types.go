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
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource/resourcerest"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource/resourcestrategy"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Extension defines an extension that's evaluated on Tilt startup.
// +k8s:openapi-gen=true
// +tilt:starlark-gen=true
type Extension struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   ExtensionSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status ExtensionStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// ExtensionList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ExtensionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []Extension `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ExtensionSpec defines the desired state of Extension
type ExtensionSpec struct {
	// RepoName specifies the ExtensionRepo object where we should find this extension.
	//
	// The Extension controller should watch for changes to this repo, and
	// may update if this repo is deleted or moved.
	RepoName string `json:"repoName" protobuf:"bytes,1,opt,name=repoName"`

	// RepoPath specifies the path to the extension directory inside the repo.
	//
	// Once the repo is downloaded, this path should point to a directory with a
	// Tiltfile as the main "entrypoint" of the extension.
	RepoPath string `json:"repoPath" protobuf:"bytes,2,opt,name=repoPath"`

	// Arguments to the Tiltfile loaded by this extension.
	//
	// Arguments can be positional (['a', 'b', 'c']) or flag-based ('--to-edit=a').
	// By default, a list of arguments indicates the list of services in the tiltfile
	// that should be enabled.
	//
	// +optional
	Args []string `json:"args,omitempty" protobuf:"bytes,3,rep,name=args"`
}

var _ resource.Object = &Extension{}
var _ resourcerest.SingularNameProvider = &Extension{}
var _ resourcestrategy.Validater = &Extension{}

func (in *Extension) GetSingularName() string {
	return "extension"
}

func (in *Extension) GetSpec() interface{} {
	return in.Spec
}

func (in *Extension) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *Extension) NamespaceScoped() bool {
	return false
}

func (in *Extension) ShortNames() []string {
	return []string{"ext"}
}

func (in *Extension) New() runtime.Object {
	return &Extension{}
}

func (in *Extension) NewList() runtime.Object {
	return &ExtensionList{}
}

func (in *Extension) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "extensions",
	}
}

func (in *Extension) IsStorageVersion() bool {
	return true
}

func (in *Extension) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &ExtensionList{}

func (in *ExtensionList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// ExtensionStatus defines the observed state of Extension
type ExtensionStatus struct {
	// Contains information about any problems loading the extension.
	Error string `json:"error,omitempty" protobuf:"bytes,1,opt,name=error"`

	// The path to the extension on disk. This location should be shared
	// and readable by all Tilt instances.
	Path string `json:"path,omitempty" protobuf:"bytes,2,opt,name=path"`
}

// Extension implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &Extension{}

func (in *Extension) GetStatus() resource.StatusSubResource {
	return in.Status
}

// ExtensionStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &ExtensionStatus{}

func (in ExtensionStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*Extension).Status = in
}
