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

// FileWatch
// +k8s:openapi-gen=true
type FileWatch struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   FileWatchSpec   `json:"spec,omitempty"`
	Status FileWatchStatus `json:"status,omitempty"`
}

// FileWatchList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type FileWatchList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []FileWatch `json:"items"`
}

// FileWatchSpec defines the desired state of FileWatch
type FileWatchSpec struct {
}

var _ resource.Object = &FileWatch{}
var _ resourcestrategy.Validater = &FileWatch{}

func (in *FileWatch) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *FileWatch) NamespaceScoped() bool {
	return false
}

func (in *FileWatch) New() runtime.Object {
	return &FileWatch{}
}

func (in *FileWatch) NewList() runtime.Object {
	return &FileWatchList{}
}

func (in *FileWatch) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "core.tilt.dev",
		Version:  "v1alpha1",
		Resource: "fileWatchs",
	}
}

func (in *FileWatch) IsStorageVersion() bool {
	return true
}

func (in *FileWatch) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &FileWatchList{}

func (in *FileWatchList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// FileWatchStatus defines the observed state of FileWatch
type FileWatchStatus struct {
}

// FileWatch implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &FileWatch{}

func (in *FileWatch) GetStatus() resource.StatusSubResource {
	return in.Status
}

// FileWatchStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &FileWatchStatus{}

func (in FileWatchStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*FileWatch).Status = in
}
