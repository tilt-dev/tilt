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
	"path"

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

// LiveUpdate
// +k8s:openapi-gen=true
type LiveUpdate struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   LiveUpdateSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status LiveUpdateStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// LiveUpdateList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type LiveUpdateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []LiveUpdate `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// LiveUpdateSpec defines the desired state of LiveUpdate
type LiveUpdateSpec struct {
	// An absolute local path that serves as the basis for all
	// path calculations.
	//
	// Relative paths in this object are calculated relative to the base path. It
	// cannot be empty.
	//
	// +tilt:local-path=true
	BasePath string `json:"basePath" protobuf:"bytes,1,opt,name=basePath"`

	// Specifies how this live-updater finds the containers that need live update.
	Selector LiveUpdateSelector `json:"selector" protobuf:"bytes,8,opt,name=selector"`

	// Sources of files to sync.
	//
	// Every live update must be associated with at least one Source object
	// to trigger the update. Usually, Tilt structures it so that there's
	// a Source for each image we depend on.
	Sources []LiveUpdateSource `json:"sources,omitempty" protobuf:"bytes,9,rep,name=sources"`

	// A list of relative paths that will immediately stop the live-update for the
	// current container.
	//
	// Used to detect file changes that invalidate the entire container image,
	// forcing a complete rebuild.
	//
	// +optional
	StopPaths []string `json:"stopPaths,omitempty" protobuf:"bytes,4,rep,name=stopPaths"`

	// Specify paths that can be live-updated into the container and their destinations.
	// Any file changes observed that do not match any of these will invalidate the container image and force a complete rebuild.
	//
	// +optional
	Syncs []LiveUpdateSync `json:"syncs,omitempty" protobuf:"bytes,5,rep,name=syncs"`

	// A list of commands to run inside the container after files are synced.
	//
	// NB: In some documentation, we call these 'runs'. 'exec' more clearly
	// matches kubectl exec for remote commands.
	//
	// +optional
	Execs []LiveUpdateExec `json:"execs,omitempty" protobuf:"bytes,6,rep,name=execs"`

	// Specifies whether Tilt should try to natively restart the container in-place
	// after syncs and execs.
	//
	// Note that native restarts are only supported by Docker and Docker Compose
	// (and NOT docker-shim or containerd, the most common Kubernetes runtimes).
	//
	// To restart on live-update in Kubernetes, see the guide for how
	// to apply extensions to add restart behavior:
	//
	// https://docs.tilt.dev/live_update_reference.html
	//
	// +optional
	Restart LiveUpdateRestartStrategy `json:"restart,omitempty" protobuf:"bytes,7,opt,name=restart,casttype=LiveUpdateRestartStrategy"`
}

var _ resource.Object = &LiveUpdate{}
var _ resourcestrategy.Validater = &LiveUpdate{}

func (in *LiveUpdate) GetSpec() interface{} {
	return in.Spec
}

func (in *LiveUpdate) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *LiveUpdate) NamespaceScoped() bool {
	return false
}

func (in *LiveUpdate) New() runtime.Object {
	return &LiveUpdate{}
}

func (in *LiveUpdate) NewList() runtime.Object {
	return &LiveUpdateList{}
}

func (in *LiveUpdate) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "liveupdates",
	}
}

func (in *LiveUpdate) IsStorageVersion() bool {
	return true
}

func (in *LiveUpdate) Validate(ctx context.Context) field.ErrorList {
	errors := field.ErrorList{}
	if len(in.Spec.Syncs) == 0 && len(in.Spec.Execs) == 0 {
		errors = append(errors,
			field.Invalid(
				field.NewPath("spec.syncs"),
				in.Spec.Syncs,
				"must contain at least 1 sync or 1 exec to run on live update"))
	}

	for i, sync := range in.Spec.Syncs {
		// We assume a Linux container, and so use `path` to check that
		// the sync dest is a LINUX abs path! (`filepath.IsAbs` varies depending on
		// OS the binary was installed for; `path` deals with Linux paths only.)
		if !path.IsAbs(sync.ContainerPath) {
			errors = append(errors,
				field.Invalid(
					field.NewPath("spec.syncs").Index(i),
					sync.ContainerPath,
					"sync destination is not absolute"))
		}
	}

	selectorPath := field.NewPath("spec.selector")
	kSelector := in.Spec.Selector.Kubernetes
	dcSelector := in.Spec.Selector.DockerCompose
	if kSelector != nil {
		p := selectorPath.Child("kubernetes")
		if kSelector.DiscoveryName == "" {
			errors = append(errors, field.Required(p.Child("discoveryName"), "KubernetesDiscovery name is required"))
		}
		var selectorFields []*field.Path
		if kSelector.Image != "" {
			selectorFields = append(selectorFields, p.Child("image"))
		}
		if kSelector.ContainerName != "" {
			selectorFields = append(selectorFields, p.Child("containerName"))
		}
		if kSelector.ImageMap != "" {
			selectorFields = append(selectorFields, p.Child("imageMap"))
		}
		if len(selectorFields) == 0 {
			errors = append(errors, field.Required(p, "exactly one of image, imageMap, or containerName is required"))
		} else if len(selectorFields) > 1 {
			for _, f := range selectorFields {
				errors = append(errors,
					field.Forbidden(f, "cannot specify more than one of image, imageMap, or containerName"))
			}
		}
	} else if dcSelector != nil {
		p := selectorPath.Child("dockerCompose")
		if dcSelector.Service == "" {
			errors = append(errors, field.Required(p.Child("service"), "DockerCompose service name is required"))
		}
	}

	return errors
}

var _ resource.ObjectList = &LiveUpdateList{}

func (in *LiveUpdateList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// LiveUpdateStatus defines the observed state of LiveUpdate
type LiveUpdateStatus struct {
	// A list of all containers that the live-updater is currently connected to.
	// +optional
	Containers []LiveUpdateContainerStatus `json:"containers,omitempty" protobuf:"bytes,1,rep,name=containers"`

	// If any of the containers are currently failing to process updates,
	// the Failed state surfaces information about what's happening and what
	// the live-updater is doing to fix the problem.
	//
	// If all containers are updating successfully, Failed will be nil.
	//
	// +optional
	Failed *LiveUpdateStateFailed `json:"failed,omitempty" protobuf:"bytes,2,opt,name=failed"`
}

// LiveUpdate implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &LiveUpdate{}

func (in *LiveUpdate) GetStatus() resource.StatusSubResource {
	return in.Status
}

// LiveUpdateStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &LiveUpdateStatus{}

func (in LiveUpdateStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*LiveUpdate).Status = in
}

// Specifies how to pull in files.
type LiveUpdateSource struct {
	// The name of a FileWatch to use as a file source.
	//
	// +optional
	FileWatch string `json:"fileWatch,omitempty" protobuf:"bytes,1,opt,name=fileWatch"`

	// Name of the ImageMap object to watch for which file changes from this source
	// are included in the container image.
	//
	// If not provided, the live-updater will copy any file changes that it's aware of,
	// even if they're already included in the container.
	//
	// +optional
	ImageMap string `json:"imageMap,omitempty" protobuf:"bytes,2,opt,name=imageMap"`
}

// Specifies how to select containers to live update.
//
// Every live update must be associated with some object for finding
// containers. In the future, we expect there to be other types
// of container discovery objects (like Docker Compose container discovery).
type LiveUpdateSelector struct {
	// Finds containers in Kubernetes.
	Kubernetes *LiveUpdateKubernetesSelector `json:"kubernetes,omitempty" protobuf:"bytes,1,opt,name=kubernetes"`

	// Finds containers in Docker Compose.
	DockerCompose *LiveUpdateDockerComposeSelector `json:"dockerCompose,omitempty" protobuf:"bytes,2,opt,name=dockerCompose"`
}

// Specifies how to select containers to live update inside K8s.
type LiveUpdateKubernetesSelector struct {
	// The name of a KubernetesDiscovery object for finding pods.
	DiscoveryName string `json:"discoveryName" protobuf:"bytes,1,opt,name=discoveryName"`

	// ApplyName is the name of a KubernetesApply object for filtering discovered pods to prevent updating old
	// deployments.
	//
	// If not provided, no filtering will be applied and all discovered Pods will be eligible for update.
	//
	// +optional
	ApplyName string `json:"applyName,omitempty" protobuf:"bytes,3,opt,name=applyName"`

	// Image specifies the name of the image that we're copying files into.
	//
	// Determines which containers in a pod to live-update.
	// Matches images by name unless tag is explicitly specified.
	//
	// Exactly one of Image, ContainerName, or ImageMap MUST be specified.
	//
	// +optional
	Image string `json:"image,omitempty" protobuf:"bytes,2,opt,name=image"`

	// ContainerName specifies the name of the container that we're copying files into.
	//
	// Exactly one of Image, ContainerName, or ImageMap MUST be specified.
	//
	// +optional
	ContainerName string `json:"containerName,omitempty" protobuf:"bytes,4,opt,name=containerName"`

	// ImageMap specifies the name of an ImageMap object to use for determining
	// the image we're copying files into.
	//
	// Exactly one of Image, ContainerName, or ImageMap MUST be specified.
	//
	// +optional
	ImageMap string `json:"imageMap,omitempty" protobuf:"bytes,5,opt,name=imageMap"`
}

// Specifies how to select containers to live update inside Docker Compose.
type LiveUpdateDockerComposeSelector struct {
	// The name of a DockerComposeService object.
	//
	// For simple projects, this is usually the same as the service
	// name in the docker-compose.yml file. (But it doesn't necessarily
	// have to be.)
	Service string `json:"service" protobuf:"bytes,1,opt,name=service"`
}

// Determines how a local path maps into a container image.
type LiveUpdateSync struct {
	// A relative path to local files. Required.
	//
	// Computed relative to the live-update BasePath.
	LocalPath string `json:"localPath" protobuf:"bytes,1,opt,name=localPath"`

	// An absolute path inside the container. Required.
	ContainerPath string `json:"containerPath" protobuf:"bytes,2,opt,name=containerPath"`
}

// Runs a remote command after files have been synced to the container.
// Commonly used for small in-container changes (like moving files
// around, or restart processes).
type LiveUpdateExec struct {
	// Command-line arguments to run inside the container. Must have length at least 1.
	Args []string `json:"args" protobuf:"bytes,1,rep,name=args"`

	// A list of relative paths that trigger this command exec.
	//
	// If not specified, all file changes seen by the LiveUpdate trigger this exec.
	//
	// Paths are specified relative to the the BasePath of the LiveUpdate.
	//
	// +optional
	TriggerPaths []string `json:"triggerPaths" protobuf:"bytes,2,rep,name=triggerPaths"`
}

// Specifies whether Tilt should try to natively restart the container in-place
// after syncs and execs.
//
// Note that native restarts are only supported by Docker and Docker Compose
// (and NOT docker-shim or containerd, the most common Kubernetes runtimes).
//
// To restart on live-update in Kubernetes, see the guide for how
// to apply extensions to add restart behavior:
//
// https://docs.tilt.dev/live_update_reference.html
type LiveUpdateRestartStrategy string

var (
	// Never use native restarts.
	LiveUpdateRestartStrategyNone LiveUpdateRestartStrategy = "none"

	// Always try to restart the container.
	//
	// If you're connected to a container runtime that does not support native
	// restarts, this will be an error.
	LiveUpdateRestartStrategyAlways LiveUpdateRestartStrategy = "always"
)

// LiveUpdateContainerStatus defines the observed state of
// the live-update syncer for a particular container.
type LiveUpdateContainerStatus struct {
	// The name of the container in the pod.
	ContainerName string `json:"containerName" protobuf:"bytes,1,opt,name=containerName"`

	// The ID of the container in the pod, in the format 'docker://<container_id>'.
	// +optional
	ContainerID string `json:"containerID,omitempty" protobuf:"bytes,2,opt,name=containerID"`

	// The name of the pod this container belongs to.
	PodName string `json:"podName" protobuf:"bytes,3,opt,name=podName"`

	// The namespace of the pod this container belongs to.
	Namespace string `json:"namespace" protobuf:"bytes,4,opt,name=namespace"`

	// The timestamp of the most recent file update successfully synced to the
	// container.
	//
	// Must match the timestamp in a FileEvent, not the time the sync was performed.
	//
	// +optional
	LastFileTimeSynced metav1.MicroTime `json:"lastFileTimeSynced,omitempty" protobuf:"bytes,5,opt,name=lastFileTimeSynced"`

	// Contains any error messages from the most recent sequence of Execs.
	//
	// Empty if the most recent Execs completed successfully.
	//
	// An ExecError is not necessarily a failure state. For example, a linter
	// error in the container is something we'd want to surface to the user, but
	// not an indication that the live-updater did something wrong.
	//
	// +optional
	LastExecError string `json:"lastExecError,omitempty" protobuf:"bytes,6,opt,name=lastExecError"`

	// Details about a waiting live update.
	//
	// A live update is waiting when the reconciler is aware of file changes
	// that need to be synced to the container, but has decided not to sync them yet.
	Waiting *LiveUpdateContainerStateWaiting `json:"waiting,omitempty" protobuf:"bytes,7,opt,name=waiting"`
}

// If any of the containers are currently failing to process updates, the
// LiveUpdateStateFailed surfaces information about what's happening and what
// the live-updater is doing to fix the problem.
type LiveUpdateStateFailed struct {
	// One word camel-case reason why we've reached a failure state.
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,1,opt,name=reason"`

	// Human-readable description of what's wrong.
	// +optional
	Message string `json:"message,omitempty" protobuf:"bytes,2,opt,name=message"`

	// When the live-updater transitioned into a Failed state.
	// +optional
	LastTransitionTime metav1.MicroTime `json:"lastTransitionTime,omitempty" protobuf:"bytes,3,opt,name=lastTransitionTime"`
}

type LiveUpdateContainerStateWaiting struct {
	// One word camel-case reason why we're in a waiting state.
	// +optional
	Reason string `json:"reason,omitempty" protobuf:"bytes,1,opt,name=reason"`

	// Human-readable description of what's blocking.
	// +optional
	Message string `json:"message,omitempty" protobuf:"bytes,2,opt,name=message"`
}
