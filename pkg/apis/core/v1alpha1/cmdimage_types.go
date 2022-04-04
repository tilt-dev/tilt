/*
Copyright 2021 The Tilt Dev Authors

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

// CmdImage describes an image to build with an arbitrary shell command.
// +k8s:openapi-gen=true
type CmdImage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   CmdImageSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status CmdImageStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// CmdImageList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CmdImageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []CmdImage `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// CmdImageSpec describes how the custom script builds images and where it puts them.
type CmdImageSpec struct {
	// The named reference of the image.
	Ref string `json:"ref" protobuf:"bytes,7,opt,name=ref"`

	// Command-line arguments. Must have length at least 1.
	Args []string `json:"args,omitempty" protobuf:"bytes,1,rep,name=args"`

	// Process working directory.
	//
	// If the working directory is not specified, the command is run
	// in the default Tilt working directory.
	//
	// +optional
	// +tilt:local-path=true
	Dir string `json:"dir,omitempty" protobuf:"bytes,2,opt,name=dir"`

	// Names of image maps that this build depends on.
	//
	// The controller will watch all the image maps, and rebuild the image
	// if any of the maps resolve to a new image.
	//
	// +optional
	ImageMaps []string `json:"imageMaps,omitempty" protobuf:"bytes,3,rep,name=imageMaps"`

	// Specifies where the image is built. If not specified, we assume the image
	// was built to the local Docker image store.
	OutputMode CmdImageOutputMode `json:"outputMode,omitempty" protobuf:"bytes,4,opt,name=outputMode,casttype=CmdImageOutputMode"`

	// Tag we expect the image to be built with (we use this to check that
	// the expected image+tag has been created).
	//
	// If empty, we create an expected tag at the beginning of CustomBuild (and
	// export $EXPECTED_REF=name:expected_tag )
	//
	// +optional
	OutputTag string `json:"outputTag,omitempty" protobuf:"bytes,5,opt,name=outputTag"`

	// Specifies a filepath where the cmd script prints the result image ref.
	//
	// Tilt will read it out when we're done to find the image.
	//
	// +optional
	// +tilt:local-path=true
	OutputsImageRefTo string `json:"outputsImageRefTo,omitempty" protobuf:"bytes,6,opt,name=outputsImageRefTo"`

	// The name of the cluster we're building for.
	//
	// We'll use the cluster to determine the architecture of the image to build,
	// and the registry to build it for.
	//
	// If no cluster is specified, assumes the default cluster.
	//
	// +optional
	Cluster string `json:"cluster,omitempty" protobuf:"bytes,8,opt,name=cluster"`

	// Whether the cluster needs access to the image.
	//
	// If not specified, assumes we have to push up to the cluster.
	//
	// +optional
	ClusterNeeds ClusterImageNeeds `json:"clusterNeeds,omitempty" protobuf:"bytes,9,opt,name=clusterNeeds,casttype=ClusterImageNeeds"`
}

var _ resource.Object = &CmdImage{}
var _ resourcestrategy.Validater = &CmdImage{}

func (in *CmdImage) GetSpec() interface{} {
	return in.Spec
}

func (in *CmdImage) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *CmdImage) NamespaceScoped() bool {
	return false
}

func (in *CmdImage) New() runtime.Object {
	return &CmdImage{}
}

func (in *CmdImage) NewList() runtime.Object {
	return &CmdImageList{}
}

func (in *CmdImage) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "cmdimages",
	}
}

func (in *CmdImage) IsStorageVersion() bool {
	return true
}

func (in *CmdImage) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &CmdImageList{}

func (in *CmdImageList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// CmdImageStatus describes the result of the image build.
type CmdImageStatus struct {
	// A fully-qualified image reference of a built image, as seen from the local
	// network.
	//
	// Usually includes a name and an immutable tag.
	//
	// NB: If we're building to a particular registry, this may
	// have a different hostname from the Spec `Ref` field.
	//
	// +optional
	Ref string `json:"ref,omitempty" protobuf:"bytes,1,opt,name=ref"`

	// Details about a waiting image build.
	// +optional
	Waiting *CmdImageStateWaiting `json:"waiting,omitempty" protobuf:"bytes,2,opt,name=waiting"`

	// Details about a building image.
	// +optional
	Building *CmdImageStateBuilding `json:"building,omitempty" protobuf:"bytes,3,opt,name=building"`

	// Details about a finished image build.
	// +optional
	Completed *CmdImageStateCompleted `json:"completed,omitempty" protobuf:"bytes,4,opt,name=completed"`
}

// CmdImage implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &CmdImage{}

func (in *CmdImage) GetStatus() resource.StatusSubResource {
	return in.Status
}

// CmdImageStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &CmdImageStatus{}

func (in CmdImageStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*CmdImage).Status = in
}

// CmdImageOutputMode describes places where the image may be written.
type CmdImageOutputMode string

const (
	// Written to the Docker image store only.
	CmdImageOutputLocalDocker CmdImageOutputMode = "local-docker"

	// Written to the Docker image store and pushed to the remote
	// destination.
	CmdImageOutputLocalDockerAndRemote CmdImageOutputMode = "local-docker-and-remote"

	// Written directly to the remote destination.
	CmdImageOutputRemote CmdImageOutputMode = "remote"
)

// CmdImageStateWaiting expresses what we're waiting on to build an image.
type CmdImageStateWaiting struct {
	// (brief) reason the image build is waiting.
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,1,opt,name=reason"`
}

// CmdImageStateBuilding expresses that an image build is in-progress.
type CmdImageStateBuilding struct {
	// The reason why the image is building.
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,1,opt,name=reason"`

	// Time when the build started.
	StartedAt metav1.MicroTime `json:"startedAt,omitempty" protobuf:"bytes,2,opt,name=startedAt"`
}

// CmdImageStateCompleted expresses when the image build is finished and
// no new images need to be built.
type CmdImageStateCompleted struct {
	// The reason why the image was built.
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,1,opt,name=reason"`

	// Error message if the build failed.
	// +optional
	Error string `json:"error,omitempty" protobuf:"bytes,2,opt,name=error"`

	// Time when we started building an image.
	StartedAt metav1.MicroTime `json:"startedAt,omitempty" protobuf:"bytes,3,opt,name=startedAt"`

	// Time when we finished building an image
	FinishedAt metav1.MicroTime `json:"finishedAt,omitempty" protobuf:"bytes,4,opt,name=finishedAt"`
}
