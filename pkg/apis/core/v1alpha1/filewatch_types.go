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

// FileWatch
// +k8s:openapi-gen=true
type FileWatch struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   FileWatchSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status FileWatchStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// FileWatchList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type FileWatchList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []FileWatch `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// FileWatchSpec defines the desired state of FileWatch
type FileWatchSpec struct {
	// WatchedPaths are paths of directories or files to watch for changes to. It cannot be empty.
	WatchedPaths []string `json:"watchedPaths" protobuf:"bytes,1,rep,name=watchedPaths"`
	// Ignores are optional rules to filter out a subset of changes matched by WatchedPaths.
	Ignores []IgnoreDef `json:"ignores,omitempty" protobuf:"bytes,2,rep,name=ignores"`
}

type IgnoreDef struct {
	// BasePath is the base path for the patterns. It cannot be empty.
	//
	// If no patterns are specified, everything under it will be recursively ignored.
	BasePath string `json:"basePath" protobuf:"bytes,1,opt,name=basePath"`
	// Patterns are dockerignore style rules. Absolute-style patterns will be rooted to the BasePath.
	//
	// See https://docs.docker.com/engine/reference/builder/#dockerignore-file.
	Patterns []string `json:"patterns,omitempty" protobuf:"bytes,2,rep,name=patterns"`
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
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "filewatches",
	}
}

func (in *FileWatch) IsStorageVersion() bool {
	return true
}

func (in *FileWatch) Validate(_ context.Context) field.ErrorList {
	var fieldErrors field.ErrorList
	if len(in.Spec.WatchedPaths) == 0 {
		fieldErrors = append(fieldErrors, field.Required(
			field.NewPath("spec", "watchedPaths"),
			"cannot be an empty list"))
	}
	return fieldErrors
}

var _ resource.ObjectList = &FileWatchList{}

func (in *FileWatchList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// FileWatchStatus defines the observed state of FileWatch
type FileWatchStatus struct {
	// MonitorStartTime is the timestamp of when filesystem monitor was started. It is zero if the monitor has not
	// been started yet.
	MonitorStartTime metav1.MicroTime `json:"monitorStartTime,omitempty" protobuf:"bytes,1,opt,name=monitorStartTime"`
	// LastEventTime is the timestamp of the most recent file event. It is zero if no events have been seen yet.
	//
	// If the specifics of which files changed are not important, this field can be used as a watermark without
	// needing to inspect FileEvents.
	LastEventTime metav1.MicroTime `json:"lastEventTime,omitempty" protobuf:"bytes,2,opt,name=lastEventTime"`
	// FileEvents summarizes batches of file changes (create, modify, or delete) that have been seen in ascending
	// chronological order. Only the most recent 20 events are included.
	FileEvents []FileEvent `json:"fileEvents,omitempty" protobuf:"bytes,3,rep,name=fileEvents"`
	// Error is set if there is a problem with the filesystem watch. If non-empty, consumers should assume that
	// no filesystem events will be seen and that the file watcher is in a failed state.
	Error string `json:"error,omitempty" protobuf:"bytes,4,opt,name=error"`
}

type FileEvent struct {
	// Time is an approximate timestamp for a batch of file changes.
	//
	// This will NOT exactly match any inode attributes (e.g. ctime, mtime) at the filesystem level and is purely
	// informational or for use as an opaque watermark.
	Time metav1.MicroTime `json:"time" protobuf:"bytes,1,opt,name=time"`
	// SeenFiles is a list of paths which changed (create, modify, or delete).
	SeenFiles []string `json:"seenFiles" protobuf:"bytes,2,rep,name=seenFiles"`
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
