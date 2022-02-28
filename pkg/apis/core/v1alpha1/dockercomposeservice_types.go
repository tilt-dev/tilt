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
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource/resourcerest"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource/resourcestrategy"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DockerComposeService represents a container orchestrated by Docker Compose.
//
// +k8s:openapi-gen=true
type DockerComposeService struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   DockerComposeServiceSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status DockerComposeServiceStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// DockerComposeServiceList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DockerComposeServiceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []DockerComposeService `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// DockerComposeServiceSpec defines the desired state a Docker Compose container.
type DockerComposeServiceSpec struct {
	// The name of the service to create.
	Service string `json:"service" protobuf:"bytes,1,opt,name=service"`

	// A specification of the project the service belongs to.
	//
	// Each service spec keeps its own copy of the project spec.
	Project DockerComposeProject `json:"project" protobuf:"bytes,2,opt,name=project"`

	// The image maps that this deploy depends on.
	ImageMaps []string `json:"imageMaps,omitempty" protobuf:"bytes,3,rep,name=imageMaps"`
}

var _ resource.Object = &DockerComposeService{}
var _ resourcestrategy.Validater = &DockerComposeService{}
var _ resourcerest.ShortNamesProvider = &DockerComposeService{}

func (in *DockerComposeService) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *DockerComposeService) NamespaceScoped() bool {
	return false
}

func (in *DockerComposeService) ShortNames() []string {
	return []string{"dc"}
}

func (in *DockerComposeService) New() runtime.Object {
	return &DockerComposeService{}
}

func (in *DockerComposeService) NewList() runtime.Object {
	return &DockerComposeServiceList{}
}

func (in *DockerComposeService) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "dockercomposeservices",
	}
}

func (in *DockerComposeService) IsStorageVersion() bool {
	return true
}

func (in *DockerComposeService) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &DockerComposeServiceList{}

func (in *DockerComposeServiceList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// DockerComposeServiceStatus defines the observed state of DockerComposeService
type DockerComposeServiceStatus struct {
}

// DockerComposeService implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &DockerComposeService{}

func (in *DockerComposeService) GetStatus() resource.StatusSubResource {
	return in.Status
}

// DockerComposeServiceStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &DockerComposeServiceStatus{}

func (in DockerComposeServiceStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*DockerComposeService).Status = in
}

type DockerComposeProject struct {
	// Configuration files to load.
	//
	// If both ConfigPaths and ProjectPath/YAML are specified,
	// the YAML is the source of truth, and the ConfigPaths
	// are used to print diagnostic information.
	ConfigPaths []string `json:"configPaths,omitempty" protobuf:"bytes,1,rep,name=configPaths"`

	// The base path of the docker-compose project.
	//
	// Expressed in docker-compose as --project-directory.
	//
	// When used on the command-line, the Docker Compose spec mandates that this
	// must be the directory of the first yaml file.  All additional yaml files are
	// evaluated relative to this project path.
	ProjectPath string `json:"projectPath,omitempty" protobuf:"bytes,2,opt,name=projectPath"`

	// The docker-compose config YAML.
	//
	// Usually contains multiple services.
	//
	// If you have multiple docker-compose.yaml files, you can combine them into a
	// single YAML with `docker-compose -f file1.yaml -f file2.yaml config`.
	YAML string `json:"yaml,omitempty" protobuf:"bytes,3,opt,name=yaml"`

	// The docker-compose project name.
	//
	// If omitted, the default is to use the NormalizedName of the ProjectPath
	// base name.
	Name string `json:"name,omitempty" protobuf:"bytes,4,opt,name=name"`

	// Path to an env file to use. Passed to docker-compose as `--env-file FILE`.
	EnvFile string `json:"envFile,omitempty" protobuf:"bytes,5,opt,name=envFile"`
}
