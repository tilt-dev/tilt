/*
Copyright 2021, 2022 The Tilt Dev Authors

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

const ClusterNameDefault = "default"
const ClusterNameDocker = "docker"

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Cluster defines any runtime for running containers, in the broadest
// sense of the word "runtime".
//
// +k8s:openapi-gen=true
type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   ClusterSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status ClusterStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// ClusterList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []Cluster `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// ClusterSpec defines how to find the cluster we're running
// containers on.
//
// Tilt currently supports connecting to an existing Kubernetes
// cluster or an existing Docker Daemon (for Docker Compose).
type ClusterSpec struct {
	// Connection spec for an existing cluster.
	Connection *ClusterConnection `json:"connection,omitempty" protobuf:"bytes,1,opt,name=connection"`
}

// Connection spec for an existing cluster.
type ClusterConnection struct {
	// Defines connection to a Kubernetes cluster.
	Kubernetes *KubernetesClusterConnection `json:"kubernetes,omitempty" protobuf:"bytes,1,opt,name=kubernetes"`

	// Defines connection to a Docker daemon.
	Docker *DockerClusterConnection `json:"docker,omitempty" protobuf:"bytes,2,opt,name=docker"`
}

type KubernetesClusterConnection struct {
	// The name of the kubeconfig context to use.
	//
	// If not specified, will use the default context in the kubeconfig.
	Context string `json:"context,omitempty" protobuf:"bytes,1,opt,name=context"`

	// The default namespace to use.
	//
	// If not specified, will use the namespace in the kubeconfig.
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,2,opt,name=namespace"`
}

type DockerClusterConnection struct {
	// The docker host to use.
	//
	// If not specified, will read the DOCKER_HOST env or use the default docker
	// host.
	Host string `json:"host,omitempty" protobuf:"bytes,1,opt,name=host"`
}

var _ resource.Object = &Cluster{}
var _ resourcestrategy.Validater = &Cluster{}

func (in *Cluster) GetSpec() interface{} {
	return in.Spec
}

func (in *Cluster) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *Cluster) NamespaceScoped() bool {
	return false
}

func (in *Cluster) New() runtime.Object {
	return &Cluster{}
}

func (in *Cluster) NewList() runtime.Object {
	return &ClusterList{}
}

func (in *Cluster) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "clusters",
	}
}

func (in *Cluster) IsStorageVersion() bool {
	return true
}

func (in *Cluster) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &ClusterList{}

func (in *ClusterList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// ClusterStatus defines the observed state of Cluster
type ClusterStatus struct {
	// The preferred chip architecture of the cluster.
	//
	// On Kubernetes, this will correspond to the kubernetes.io/arch annotation on
	// a node.
	//
	// On Docker, this will be the Architecture of the Docker daemon.
	//
	// Note that many clusters support multiple chipsets. This field doesn't intend
	// that this is the only architecture a cluster supports, only that it's one
	// of the architectures.
	Arch string `json:"arch,omitempty" protobuf:"bytes,1,opt,name=arch"`

	// An unrecoverable error connecting to the cluster.
	//
	// +optional
	Error string `json:"error,omitempty" protobuf:"bytes,2,opt,name=error"`

	// ConnectedAt indicates the time at which the cluster connection was established.
	//
	// Consumers can use this to detect when the underlying config has changed
	// and refresh their client/connection accordingly.
	//
	// +optional
	ConnectedAt *metav1.MicroTime `json:"connectedAt,omitempty" protobuf:"bytes,3,opt,name=connectedAt"`
}

// Cluster implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &Cluster{}

func (in *Cluster) GetStatus() resource.StatusSubResource {
	return in.Status
}

// ClusterStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &ClusterStatus{}

func (in ClusterStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*Cluster).Status = in
}
