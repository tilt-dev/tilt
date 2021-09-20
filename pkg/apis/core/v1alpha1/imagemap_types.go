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

// ImageMap expresses the mapping from an image reference to a real, pushed
// image in an image registry that a container runtime can access.
//
// Another way to think about the ImageMap is that ImageMapSpec is a mutable
// image reference (where the image might not exist yet), but ImageMapStatus is
// an immutable image reference (where, if an image is specified, it always
// exists).
//
// ImageMap does not specify how the image is built or who is responsible for
// building this. But any API that builds images should produce an ImageMap.
//
// For example, a builder that builds to a local image registry might create
// a map from: 'my-apiserver:dev' to 'localhost:5000/my-apiserver:content-based-label'.
//
// ImageMap doesn't follow the usual Kubernetes-style API semantics
// (where the Status is the result of running the Spec). It's closer to a
// ConfigMap. Though the Status does represent a real runtime result
// (an image in a registry).
//
// +k8s:openapi-gen=true
type ImageMap struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   ImageMapSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status ImageMapStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// ImageMapList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ImageMapList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []ImageMap `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ImageMapSpec defines the desired state of ImageMap
type ImageMapSpec struct {
	// A named image reference.
	//
	// Deployment tools expect this image reference to match an image
	// in the YAML being deployed, and will replace that image reference.
	//
	// By default, this selector will match an image if the names match
	// (tags on both the selector and the matched reference are ignored).
	Selector string `json:"selector" protobuf:"bytes,1,opt,name=selector"`

	// If specified, then tags on both the selector and the matched
	// reference are used for matching. The selector will only
	// match the reference if the tags match exactly.
	//
	// +optional
	MatchExact bool `json:"matchExact,omitempty" protobuf:"varint,2,opt,name=matchExact"`

	// If specified, then the selector will also match any strings
	// in container env variables.
	//
	// +optional
	MatchInEnvVars bool `json:"matchInEnvVars,omitempty" protobuf:"varint,3,opt,name=matchInEnvVars"`

	// If specified, the injector will replace the 'command'
	// field in the container when it replaces the image.
	//
	// +optional
	OverrideCommand *ImageMapOverrideCommand `json:"overrideCommand,omitempty" protobuf:"bytes,4,opt,name=overrideCommand"`

	// If specified, the injector will replace the 'args'
	// field in the container when it replaces the image.
	//
	// +optional
	OverrideArgs *ImageMapOverrideArgs `json:"overrideArgs,omitempty" protobuf:"bytes,5,opt,name=overrideArgs"`

	// Specifies how to disable this.
	//
	// +optional
	DisableSource *DisableSource `json:"disableSource,omitempty" protobuf:"bytes,6,opt,name=disableSource"`
}

// ImageMapCommandOverride defines a command to inject when the image
// is injected. Only applies to types that embed a v1.Container
// with a Command field.
//
// https://pkg.go.dev/k8s.io/api/core/v1#Container
type ImageMapOverrideCommand struct {
	// A list of command strings.
	Command []string `json:"command" protobuf:"bytes,1,rep,name=command"`
}

// ImageMapArgsOverride defines args to inject when the image
// is injected. Only applies to types that embed a v1.Container
// with a Command field.
//
// https://pkg.go.dev/k8s.io/api/core/v1#Container
type ImageMapOverrideArgs struct {
	// A list of args strings.
	Args []string `json:"args" protobuf:"bytes,1,rep,name=args"`
}

var _ resource.Object = &ImageMap{}
var _ resourcestrategy.Validater = &ImageMap{}
var _ resourcerest.ShortNamesProvider = &PortForward{}

func (in *ImageMap) GetSpec() interface{} {
	return in.Spec
}

func (in *ImageMap) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *ImageMap) NamespaceScoped() bool {
	return false
}

func (in *ImageMap) ShortNames() []string {
	return []string{"im"}
}

func (in *ImageMap) New() runtime.Object {
	return &ImageMap{}
}

func (in *ImageMap) NewList() runtime.Object {
	return &ImageMapList{}
}

func (in *ImageMap) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "imagemaps",
	}
}

func (in *ImageMap) IsStorageVersion() bool {
	return true
}

func (in *ImageMap) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &ImageMapList{}

func (in *ImageMapList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// ImageMapStatus defines the observed state of ImageMap
type ImageMapStatus struct {
	// A fully-qualified image reference, including a name and an immutable tag.
	//
	// The image will not necessarily have the same repo URL as the selector. Many
	// Kubernetes clusters let you push to a local registry for local development.
	Image string `json:"image" protobuf:"bytes,1,opt,name=image"`

	// TODO(nick): I'm not totally sure how we should model registries in this system.
	//
	// We need to be able to support an image existing at multiple URLs in
	// multiple registries.  Even more subtly, a registry might have multiple URLs
	// (the local registry spec has 3 - from the POV of a user outside the cluster,
	// from the POV of the cluster's container runtime, and from the POV of the
	// cluster network).
	//
	// This might mean we have multiple image references, or it might mean
	// we have one image reference that gets rebased to multiple registries.
	//
	// It might make sense for a Registry to be its own API object.
}

// ImageMap implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &ImageMap{}

func (in *ImageMap) GetStatus() resource.StatusSubResource {
	return in.Status
}

// ImageMapStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &ImageMapStatus{}

func (in ImageMapStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*ImageMap).Status = in
}
