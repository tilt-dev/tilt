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
	"path/filepath"
	strings "strings"

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

// ExtensionRepo specifies a repo or folder where a set of extensions live.
// +k8s:openapi-gen=true
type ExtensionRepo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   ExtensionRepoSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status ExtensionRepoStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// ExtensionRepoList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ExtensionRepoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []ExtensionRepo `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ExtensionRepoSpec defines how to access the repo.
type ExtensionRepoSpec struct {
	// The URL of the repo.
	//
	// Allowed:
	// https: URLs that point to a public git repo
	// file: URLs that point to a location on disk.
	URL string `json:"url" protobuf:"bytes,1,opt,name=url"`

	// A reference to sync the repo to. If empty, Tilt will always update
	// the repo to the latest version.
	// +optional
	Ref string `json:"ref,omitempty" protobuf:"bytes,2,opt,name=ref"`
}

var _ resource.Object = &ExtensionRepo{}
var _ resourcestrategy.Validater = &ExtensionRepo{}

func (in *ExtensionRepo) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *ExtensionRepo) NamespaceScoped() bool {
	return false
}

func (in *ExtensionRepo) New() runtime.Object {
	return &ExtensionRepo{}
}

func (in *ExtensionRepo) NewList() runtime.Object {
	return &ExtensionRepoList{}
}

func (in *ExtensionRepo) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "extensionrepos",
	}
}

func (in *ExtensionRepo) IsStorageVersion() bool {
	return true
}

func (in *ExtensionRepo) Validate(ctx context.Context) field.ErrorList {
	var fieldErrors field.ErrorList
	url := in.Spec.URL
	isWeb := strings.HasPrefix(url, "https://") || strings.HasPrefix(url, "http://")
	isFile := strings.HasPrefix(url, "file://")
	if !isWeb && !isFile {
		fieldErrors = append(fieldErrors, field.Invalid(
			field.NewPath("spec.url"),
			url,
			"URLs must start with http(s):// or file://"))
	} else if isFile && !filepath.IsAbs(strings.TrimPrefix(url, "file://")) {
		fieldErrors = append(fieldErrors, field.Invalid(
			field.NewPath("spec.url"),
			url,
			"file:// URLs must be absolute (e.g., file:///home/user/repo)"))
	}
	return fieldErrors
}

var _ resource.ObjectList = &ExtensionRepoList{}

func (in *ExtensionRepoList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// ExtensionRepoStatus defines the observed state of ExtensionRepo
type ExtensionRepoStatus struct {
	// Contains information about any problems loading the repo.
	Error string `json:"error,omitempty" protobuf:"bytes,1,opt,name=error"`

	// The last time the repo was fetched and checked for validity.
	LastFetchedAt metav1.Time `json:"lastFetchedAt,omitempty" protobuf:"bytes,2,opt,name=lastFetchedAt"`

	// The path to the repo on local disk.
	Path string `json:"path,omitempty" protobuf:"bytes,3,opt,name=path"`

	// The reference that we currently have checked out.
	// On git, this is the commit hash.
	// On file repos, this is empty.
	CheckoutRef string `json:"checkoutRef,omitempty" protobuf:"bytes,4,opt,name=checkoutRef"`
}

// ExtensionRepo implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &ExtensionRepo{}

func (in *ExtensionRepo) GetStatus() resource.StatusSubResource {
	return in.Status
}

// ExtensionRepoStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &ExtensionRepoStatus{}

func (in ExtensionRepoStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*ExtensionRepo).Status = in
}
