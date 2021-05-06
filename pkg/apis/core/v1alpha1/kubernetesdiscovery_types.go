/*
Copyright 2015 The Kubernetes Authors.
Copyright 2021 The Tilt Dev Authors.

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

// KubernetesDiscovery
// +k8s:openapi-gen=true
type KubernetesDiscovery struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   KubernetesDiscoverySpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status KubernetesDiscoveryStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// KubernetesDiscoveryList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type KubernetesDiscoveryList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []KubernetesDiscovery `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// KubernetesDiscoverySpec defines the desired state of KubernetesDiscovery
type KubernetesDiscoverySpec struct {
	// Watches determine what resources are discovered.
	//
	// If a discovered resource (e.g. Pod) matches the KubernetesWatchRef UID exactly, it will be reported.
	// If a discovered resource is transitively owned by the KubernetesWatchRef UID, it will be reported.
	Watches []KubernetesWatchRef `json:"watches" protobuf:"bytes,1,rep,name=watches"`

	// ExtraSelectors are label selectors that will force discovery of a Pod even if it does not match
	// the AncestorUID.
	//
	// This should only be necessary in the event that a CRD creates Pods but does not set an owner reference
	// to itself.
	ExtraSelectors []metav1.LabelSelector `json:"extraSelectors,omitempty" protobuf:"bytes,2,rep,name=extraSelectors"`
}

// KubernetesWatchRef is similar to v1.ObjectReference from the Kubernetes API and is used to determine
// what objects should be reported on based on discovery.
type KubernetesWatchRef struct {
	// UID is a Kubernetes object UID.
	//
	// It should either be the exact object UID or the transitive owner.
	UID string `json:"uid" protobuf:"bytes,1,opt,name=uid"`
	// Namespace is the Kubernetes namespace for discovery. Required.
	Namespace string `json:"namespace" protobuf:"bytes,2,opt,name=namespace"`
	// Name is the Kubernetes object name.
	//
	// This is not directly used in discovery; it is extra metadata.
	Name string `json:"name,omitempty" protobuf:"bytes,3,opt,name=name"`
}

var _ resource.Object = &KubernetesDiscovery{}
var _ resourcestrategy.Validater = &KubernetesDiscovery{}

func (in *KubernetesDiscovery) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *KubernetesDiscovery) NamespaceScoped() bool {
	return false
}

func (in *KubernetesDiscovery) New() runtime.Object {
	return &KubernetesDiscovery{}
}

func (in *KubernetesDiscovery) NewList() runtime.Object {
	return &KubernetesDiscoveryList{}
}

func (in *KubernetesDiscovery) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "kubernetesdiscoveries",
	}
}

func (in *KubernetesDiscovery) IsStorageVersion() bool {
	return true
}

func (in *KubernetesDiscovery) Validate(_ context.Context) field.ErrorList {
	var fieldErrors field.ErrorList
	watchPath := field.NewPath("spec", "watches")
	if len(in.Spec.Watches) == 0 {
		fieldErrors = append(fieldErrors, field.Required(watchPath, "One or more watches are required"))
	}
	for i := range in.Spec.Watches {
		if in.Spec.Watches[i].Namespace == "" {
			fieldErrors = append(fieldErrors, field.Required(watchPath.Index(i), "Namespace must be provided"))
		}
	}
	return fieldErrors
}

var _ resource.ObjectList = &KubernetesDiscoveryList{}

func (in *KubernetesDiscoveryList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// KubernetesDiscoveryStatus defines the observed state of KubernetesDiscovery
type KubernetesDiscoveryStatus struct {
	// MonitorStartTime is the timestamp of when Kubernetes resource discovery was started.
	//
	// It is zero if discovery has not been started yet.
	MonitorStartTime metav1.MicroTime `json:"monitorStartTime,omitempty" protobuf:"bytes,2,opt,name=monitorStartTime"`

	// Pods that have been discovered based on the criteria in the spec.
	Pods []Pod `json:"pods" protobuf:"bytes,1,rep,name=pods"`
}

// KubernetesDiscovery implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &KubernetesDiscovery{}

func (in *KubernetesDiscovery) GetStatus() resource.StatusSubResource {
	return in.Status
}

// KubernetesDiscoveryStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &KubernetesDiscoveryStatus{}

func (in KubernetesDiscoveryStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*KubernetesDiscovery).Status = in
}

// Pod is a collection of containers that can run on a host.
//
// The Tilt API representation mirrors the Kubernetes API very closely. Irrelevant data is
// not included, and some fields might be simplified.
//
// There might also be Tilt-specific status fields.
type Pod struct {
	// UID is the unique Pod UID within the K8s cluster.
	UID string `json:"uid" protobuf:"bytes,14,opt,name=uid"`
	// Name is the Pod name within the K8s cluster.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// Namespace is the Pod namespace within the K8s cluster.
	Namespace string `json:"namespace" protobuf:"bytes,2,opt,name=namespace"`
	// CreatedAt is when the Pod was created.
	CreatedAt metav1.Time `json:"createdAt" protobuf:"bytes,3,opt,name=createdAt"`
	// Phase is where the Pod is at in its current lifecycle.
	//
	// Valid values for this are v1.PodPhase values from the Kubernetes API.
	Phase string `json:"phase" protobuf:"bytes,4,opt,name=phase"`
	// Deleting indicates that the Pod is in the process of being removed.
	Deleting bool `json:"deleting" protobuf:"varint,5,opt,name=deleting"`
	// Conditions are various lifecycle conditions for this Pod.
	//
	// See also v1.PodCondition in the Kubernetes API.
	Conditions []PodCondition `json:"conditions,omitempty" protobuf:"bytes,6,rep,name=conditions"`
	// InitContainers are containers executed prior to the Pod containers being executed.
	InitContainers []Container `json:"initContainers,omitempty" protobuf:"bytes,7,rep,name=initContainers"`
	// Containers are the containers belonging to the Pod.
	Containers []Container `json:"containers" protobuf:"bytes,8,rep,name=containers"`

	// AncestorUID is the UID from the WatchRef that matched this Pod.
	//
	// If the Pod matched based on extra label selectors, this will be empty.
	//
	// +optional
	AncestorUID string `json:"ancestorUID,omitempty" protobuf:"bytes,15,opt,name=ancestorUID"`
	// BaselineRestartCount is the number of restarts across all containers before Tilt started observing the Pod.
	//
	// This is used to ignore restarts for a Pod that was already executing before the Tilt daemon started.
	BaselineRestartCount int32 `json:"baselineRestartCount" protobuf:"varint,9,opt,name=baselineRestartCount"`
	// PodTemplateSpecHash is a hash of the Pod template spec.
	//
	// Tilt uses this to associate Pods with the build that triggered them.
	PodTemplateSpecHash string `json:"podTemplateSpecHash,omitempty" protobuf:"bytes,10,opt,name=podTemplateSpecHash"`
	// UpdateStartedAt is when Tilt started a deployment update for this Pod.
	UpdateStartedAt metav1.Time `json:"updateStartedAt,omitempty" protobuf:"bytes,11,opt,name=updateStartedAt"`
	// Status is a concise description for the Pod's current state.
	//
	// This is based off the status output from `kubectl get pod` and is not an "enum-like"
	// value.
	Status string `json:"status" protobuf:"bytes,12,opt,name=status"`
	// Errors are aggregated error messages for the Pod and its containers.
	Errors []string `json:"errors" protobuf:"bytes,13,rep,name=errors"`
}

// PodCondition is a lifecycle condition for a Pod.
type PodCondition struct {
	// Type is the type of condition.
	//
	// Valid values for this are v1.PodConditionType values from the Kubernetes API.
	Type string `json:"type" protobuf:"bytes,1,opt,name=type"`
	// Status is the current state of the condition (True, False, or Unknown).
	//
	// Valid values for this are v1.PodConditionStatus values from the Kubernetes API.
	Status string `json:"status" protobuf:"bytes,2,opt,name=status"`
	// LastTransitionTime is the last time the status changed.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty" protobuf:"bytes,3,opt,name=lastTransitionTime"`
	// Reason is a unique, one-word, CamelCase value for the cause of the last status change.
	Reason string `json:"reason,omitempty" protobuf:"bytes,4,opt,name=reason"`
	// Message is a human-readable description of the last status change.
	Message string `json:"message,omitempty" protobuf:"bytes,5,opt,name=message"`
}

// Container is an init or application container within a pod.
//
// The Tilt API representation mirrors the Kubernetes API very closely. Irrelevant data is
// not included, and some fields might be simplified.
//
// There might also be Tilt-specific status fields.
type Container struct {
	// Name is the name of the container as defined in Kubernetes.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`
	// ID is the normalized container ID (the `docker://` prefix is stripped).
	ID string `json:"id" protobuf:"bytes,2,opt,name=id"`
	// Ready is true if the container is passing readiness checks (or has none defined).
	Ready bool `json:"ready" protobuf:"varint,3,opt,name=ready"`
	// Image is the image the container is running.
	Image string `json:"image" protobuf:"bytes,4,opt,name=image"`
	// Restarts is the number of times the container has restarted.
	//
	// This includes restarts before the Tilt daemon was started if the container was already running.
	Restarts int32 `json:"restarts" protobuf:"varint,5,opt,name=restarts"`
	// State provides details about the container's current condition.
	State ContainerState `json:"state" protobuf:"bytes,6,opt,name=state"`
	// Ports are exposed ports as extracted from the Pod spec.
	//
	// This is added by Tilt for convenience when managing port forwards.
	Ports []int32 `json:"ports" protobuf:"varint,7,rep,name=ports"`
}

// ContainerState holds a possible state of container.
//
// Only one of its members may be specified.
// If none of them is specified, the default one is ContainerStateWaiting.
type ContainerState struct {
	// Waiting provides details about a container that is not yet running.
	Waiting *ContainerStateWaiting `json:"waiting" protobuf:"bytes,1,opt,name=waiting"`
	// Running provides details about a currently executing container.
	Running *ContainerStateRunning `json:"running" protobuf:"bytes,2,opt,name=running"`
	// Terminated provides details about an exited container.
	Terminated *ContainerStateTerminated `json:"terminated" protobuf:"bytes,3,opt,name=terminated"`
}

// ContainerStateWaiting is a waiting state of a container.
type ContainerStateWaiting struct {
	// Reason is a (brief) reason the container is not yet running.
	Reason string `json:"reason" protobuf:"bytes,1,opt,name=reason"`
}

// ContainerStateRunning is a running state of a container.
type ContainerStateRunning struct {
	// StartedAt is the time the container began running.
	StartedAt metav1.Time `json:"startedAt" protobuf:"bytes,1,opt,name=startedAt"`
}

// ContainerStateTerminated is a terminated state of a container.
type ContainerStateTerminated struct {
	// StartedAt is the time the container began running.
	StartedAt metav1.Time `json:"startedAt" protobuf:"bytes,1,opt,name=startedAt"`
	// FinishedAt is the time the container stopped running.
	FinishedAt metav1.Time `json:"finishedAt" protobuf:"bytes,2,opt,name=finishedAt"`
	// Reason is a (brief) reason the container stopped running.
	Reason string `json:"reason,omitempty" protobuf:"bytes,3,opt,name=reason"`
	// ExitCode is the exit status from the termination of the container.
	//
	// Any non-zero value indicates an error during termination.
	ExitCode int32 `json:"exitCode" protobuf:"varint,4,opt,name=exitCode"`
}
