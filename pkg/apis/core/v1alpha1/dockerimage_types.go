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
