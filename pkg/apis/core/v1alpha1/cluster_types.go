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

	// DefaultRegistry determines where images for this Cluster should
	// be pushed/pulled from if the Cluster itself does not provide local
	// registry hosting metadata.
	//
	// If not specified, no registry rewriting will occur, and the images will
	// be pushed/pulled to from the registry specified by the corresponding
	// image build directive (e.g. `docker_build` or `custom_build`).
	//
	// +optional
	DefaultRegistry *RegistryHosting `json:"defaultRegistry,omitempty" protobuf:"bytes,2,opt,name=defaultRegistry"`
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
	//
	// +optional
	Context string `json:"context,omitempty" protobuf:"bytes,1,opt,name=context"`

	// The default namespace to use.
	//
	// If not specified, will use the namespace in the kubeconfig.
	//
	// +optional
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
	var errors field.ErrorList
	if in.Spec.DefaultRegistry != nil {
		errors = append(errors,
			in.Spec.DefaultRegistry.validateAsSubfield(ctx, field.NewPath(".spec.defaultRegistry"))...)
	}
	return errors
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

	// Registry describes a local registry that developer tools can
	// connect to. A local registry allows clients to load images into the local
	// cluster by pushing to this registry.
	//
	// +optional
	Registry *RegistryHosting `json:"registry,omitempty" protobuf:"bytes,4,opt,name=registry"`

	// Connection status for an existing cluster.
	//
	// +optional
	Connection *ClusterConnectionStatus `json:"connection,omitempty" protobuf:"bytes,5,opt,name=connection"`

	// Version is a cluster-provided, human-readable version string.
	//
	// +optional
	Version string `json:"version,omitempty" protobuf:"bytes,6,opt,name=version"`
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

// Connection spec for an existing cluster.
type ClusterConnectionStatus struct {
	// Defines connection to a Kubernetes cluster.
	Kubernetes *KubernetesClusterConnectionStatus `json:"kubernetes,omitempty" protobuf:"bytes,1,opt,name=kubernetes"`
}

// Kubernetes-specific fields for connection status
type KubernetesClusterConnectionStatus struct {
	// The resolved kubeconfig context.
	Context string `json:"context" protobuf:"bytes,2,opt,name=context"`

	// The resolved default namespace.
	Namespace string `json:"namespace" protobuf:"bytes,3,opt,name=namespace"`

	// The resolved cluster name (as determined by the kubeconfig context).
	Cluster string `json:"cluster" protobuf:"bytes,4,opt,name=cluster"`

	// The product name for this cluster.
	//
	// For a complete list of possible product names, see:
	// https://pkg.go.dev/github.com/tilt-dev/clusterid#Product
	Product string `json:"product,omitempty" protobuf:"bytes,1,opt,name=product"`

	// The resolved config path.
	//
	// Tilt will freeze the config and write it to a temporary directory.
	// Subprocesses that depend on this cluster can find this file
	// by reading the KUBECONFIG env var.
	ConfigPath string `json:"configPath,omitempty" protobuf:"bytes,5,opt,name=configPath"`
}

// ClusterImageNeeds describes the ways that a cluster
// might need to access an image.
//
// Defaults to "push".
type ClusterImageNeeds string

const (
	// Make sure the image is pushed to the right registry
	// and accessible in the cluster.
	ClusterImageNeedsPush ClusterImageNeeds = "push"

	// The image is only needed as a base image for other
	// images that are needed in the cluster, so doesn't need to be pushed.
	ClusterImageNeedsBase ClusterImageNeeds = "base-image"
)
