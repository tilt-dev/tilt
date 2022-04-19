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

// DockerImage describes an image to build with Docker.
// +k8s:openapi-gen=true
type DockerImage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   DockerImageSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status DockerImageStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// DockerImageList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DockerImageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []DockerImage `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// DockerImageSpec describes how to build a Docker image with `docker_build`.
//
// Most fields of this spec directly correspond to the Docker CLI.
type DockerImageSpec struct {
	// The named reference of the image.
	Ref string `json:"ref" protobuf:"bytes,12,opt,name=ref"`

	// Dockerfile contains the complete contents of the Dockerfile.
	//
	// TODO(nick): We should also support referencing the Dockerfile as a path.
	//
	// +optional
	DockerfileContents string `json:"dockerfileContents,omitempty" protobuf:"bytes,1,opt,name=dockerfileContents"`

	// Context specifies the Docker build context.
	//
	// Must be an absolute path on the local filesystem.
	//
	// +tilt:local-path=true
	Context string `json:"context,omitempty" protobuf:"bytes,2,opt,name=context"`

	// Ignores are filters on the Docker build context.
	//
	// The DockerImage controller will NOT read ignores from .dockerignore files.
	// Instead, all filters must be expressed in this field, which covers
	// .dockerignore files, ignore= lists in the tiltfile, only= lists in the
	// tiltfile, and more.
	ContextIgnores []IgnoreDef `json:"contextIgnores,omitempty" protobuf:"bytes,16,rep,name=contextIgnores"`

	// Args specifies the build arguments to the Dockerfile.
	//
	// Equivalent to `--build-arg` in the docker CLI.
	//
	// Each item should take the form "KEY" or "KEY=VALUE".
	//
	// +optional
	Args []string `json:"args,omitempty" protobuf:"bytes,3,rep,name=args"`

	// Target specifies the name of the stage in the Dockerfile to build.
	//
	// Equivalent to `--target` in the docker CLI.
	//
	// +optional
	Target string `json:"target,omitempty" protobuf:"bytes,4,opt,name=target"`

	// Pass SSH secrets to docker so it can clone private repos.
	//
	// https://docs.docker.com/develop/develop-images/build_enhancements/#using-ssh-to-access-private-data-in-builds
	//
	// Equivalent to `--ssh` in the docker CLI.
	//
	// +optional
	SSHAgentConfigs []string `json:"sshAgentConfigs,omitempty" protobuf:"bytes,5,rep,name=sshAgentConfigs"`

	// Pass secrets to docker.
	//
	// https://docs.docker.com/develop/develop-images/build_enhancements/#new-docker-build-secret-information
	//
	// Equivalent to `--secret` in the Docker CLI.
	//
	// +optional
	Secrets []string `json:"secrets,omitempty" protobuf:"bytes,6,rep,name=secrets"`

	// Set the networking mode for the RUN instructions in the docker build.
	//
	// Equivalent to `--network` in the Docker CLI.
	//
	// +optional
	Network string `json:"network,omitempty" protobuf:"bytes,7,opt,name=network"`

	// Always attempt to pull a new version of the base image.
	//
	// Equivalent to `--pull` in the Docker CLI.
	//
	// +optional
	Pull bool `json:"pull,omitempty" protobuf:"varint,8,opt,name=pull"`

	// Images to use as cache sources.
	//
	// Equivalent to `--cache-from` in the Docker CLI.
	CacheFrom []string `json:"cacheFrom,omitempty" protobuf:"bytes,9,rep,name=cacheFrom"`

	// Platform specifies architecture information for target image.
	//
	// https://docs.docker.com/desktop/multi-arch/
	//
	// Equivalent to `--platform` in the Docker CLI.
	Platform string `json:"platform,omitempty" protobuf:"bytes,10,opt,name=platform"`

	// By default, Tilt creates a new temporary image reference for each build.
	// The user can also specify their own reference, to integrate with other tooling
	// (like build IDs for Jenkins build pipelines)
	//
	// Equivalent to the docker build --tag flag.
	//
	// +optional
	ExtraTags []string `json:"extraTags,omitempty" protobuf:"bytes,11,rep,name=extraTags"`

	// Names of image maps that this build depends on.
	//
	// The controller will watch all the image maps, rebuild the image if any of
	// the maps resolve to a new image, and inject them into the dockerfile.
	//
	// +optional
	ImageMaps []string `json:"imageMaps,omitempty" protobuf:"bytes,13,rep,name=imageMaps"`

	// The name of the cluster we're building for.
	//
	// We'll use the cluster to determine the architecture of the image to build,
	// and the registry to build it for.
	//
	// If no cluster is specified, assumes the default cluster.
	//
	// +optional
	Cluster string `json:"cluster,omitempty" protobuf:"bytes,14,opt,name=cluster"`

	// Whether the cluster needs access to the image.
	//
	// If not specified, assumes we have to push up to the cluster.
	//
	// +optional
	ClusterNeeds ClusterImageNeeds `json:"clusterNeeds,omitempty" protobuf:"bytes,15,opt,name=clusterNeeds,casttype=ClusterImageNeeds"`
}

var _ resource.Object = &DockerImage{}
var _ resourcestrategy.Validater = &DockerImage{}

func (in *DockerImage) GetSpec() interface{} {
	return in.Spec
}

func (in *DockerImage) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *DockerImage) NamespaceScoped() bool {
	return false
}

func (in *DockerImage) New() runtime.Object {
	return &DockerImage{}
}

func (in *DockerImage) NewList() runtime.Object {
	return &DockerImageList{}
}

func (in *DockerImage) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "dockerimages",
	}
}

func (in *DockerImage) IsStorageVersion() bool {
	return true
}

func (in *DockerImage) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &DockerImageList{}

func (in *DockerImageList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// DockerImageStatus defines the observed state of DockerImage
type DockerImageStatus struct {
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
	Waiting *DockerImageStateWaiting `json:"waiting,omitempty" protobuf:"bytes,2,opt,name=waiting"`

	// Details about a building image.
	// +optional
	Building *DockerImageStateBuilding `json:"building,omitempty" protobuf:"bytes,3,opt,name=building"`

	// Details about a finished image build.
	// +optional
	Completed *DockerImageStateCompleted `json:"completed,omitempty" protobuf:"bytes,4,opt,name=completed"`

	// Status information about each individual build stage
	// of the most recent image build.
	StageStatuses []DockerImageStageStatus `json:"stageStatuses,omitempty" protobuf:"bytes,5,rep,name=stageStatuses"`
}

// DockerImage implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &DockerImage{}

func (in *DockerImage) GetStatus() resource.StatusSubResource {
	return in.Status
}

// DockerImageStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &DockerImageStatus{}

func (in DockerImageStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*DockerImage).Status = in
}

// DockerImageStateWaiting expresses what we're waiting on to build an image.
type DockerImageStateWaiting struct {
	// (brief) reason the image build is waiting.
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,1,opt,name=reason"`
}

// DockerImageStateBuilding expresses that an image build is in-progress.
type DockerImageStateBuilding struct {
	// The reason why the image is building.
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,1,opt,name=reason"`

	// Time when the build started.
	StartedAt metav1.MicroTime `json:"startedAt,omitempty" protobuf:"bytes,2,opt,name=startedAt"`
}

// DockerImageStateCompleted expresses when the image build is finished and
// no new images need to be built.
type DockerImageStateCompleted struct {
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

// DockerImageStageStatus gives detailed report of each stage
// of the most recent image build.
//
// Most stages are derived from Buildkit's StatusResponse
// https://github.com/moby/buildkit/blob/35fcb28a009d6454b2915a5c8084b25ad851cf38/api/services/control/control.proto#L108
// but Tilt may synthesize its own stages for the steps it
// owns.
//
// Stages may be executed in parallel.
type DockerImageStageStatus struct {
	// A human-readable name of the stage.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`

	// Whether Buildkit was able to cache the stage based on inputs.
	// +optional
	Cached bool `json:"cached,omitempty" protobuf:"varint,2,opt,name=cached"`

	// The timestamp when we started working on the stage.
	// +optional
	StartedAt *metav1.MicroTime `json:"startedAt,omitempty" protobuf:"bytes,6,opt,name=startedAt"`

	// The timetsamp when we completed the work on the stage.
	// +optional
	FinishedAt *metav1.MicroTime `json:"finishedAt,omitempty" protobuf:"bytes,7,opt,name=finishedAt"`

	// Error message if the stage failed. If empty, the stage succeeded.
	// +optional
	Error string `json:"error,omitempty" protobuf:"bytes,5,opt,name=error"`
}
