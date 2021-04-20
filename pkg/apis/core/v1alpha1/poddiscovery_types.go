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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Pod is a collection of containers that can run on a host.
//
// The Tilt API representation mirrors the Kubernetes API very closely. Irrelevant data is
// not included, and some fields might be simplified.
//
// There might also be Tilt-specific status fields.
type Pod struct {
	// Name is the Pod name within the K8s cluster.
	Name string `json:"name"`
	// Namespace is the Pod namespace within the K8s cluster.
	Namespace string `json:"namespace"`
	// CreatedAt is when the Pod was created.
	CreatedAt metav1.Time `json:"createdAt"`
	// Phase is where the Pod is at in its current lifecycle.
	//
	// Valid values for this are v1.PodPhase values from the Kubernetes API.
	Phase string `json:"phase"`
	// Deleting indicates that the Pod is in the process of being removed.
	Deleting bool `json:"deleting"`
	// Conditions are various lifecycle conditions for this Pod.
	//
	// See also v1.PodCondition in the Kubernetes API.
	Conditions []PodCondition `json:"conditions,omitempty"`
	// InitContainers are containers executed prior to the Pod containers being executed.
	InitContainers []Container `json:"initContainers,omitempty"`
	// Containers are the containers belonging to the Pod.
	Containers []Container `json:"containers"`

	// BaselineRestartCount is the number of restarts across all containers before Tilt started observing the Pod.
	//
	// This is used to ignore restarts for a Pod that was already executing before the Tilt daemon started.
	BaselineRestartCount int `json:"baselineRestartCount"`
	// PodTemplateSpecHash is a hash of the Pod template spec.
	//
	// Tilt uses this to associate Pods with the build that triggered them.
	PodTemplateSpecHash string `json:"podTemplateSpecHash,omitempty"`
	// UpdateStartedAt is when Tilt started a deployment update for this Pod.
	UpdateStartedAt metav1.Time `json:"updateStartedAt,omitempty"`
	// Status is a concise description for the Pod's current state.
	//
	// This is based off the status output from `kubectl get pod` and is not an "enum-like"
	// value.
	Status string `json:"status"`
	// Errors are aggregated error messages for the Pod and its containers.
	Errors []string `json:"errors"`
}

// PodCondition is a lifecycle condition for a Pod.
type PodCondition struct {
	// Type is the type of condition.
	//
	// Valid values for this are v1.PodConditionType values from the Kubernetes API.
	Type string `json:"type"`
	// Status is the current state of the condition (True, False, or Unknown).
	//
	// Valid values for this are v1.PodConditionStatus values from the Kubernetes API.
	Status string `json:"status"`
	// LastTransitionTime is the last time the status changed.
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// Reason is a unique, one-word, CamelCase value for the cause of the last status change.
	Reason string `json:"reason,omitempty"`
	// Message is a human-readable description of the last status change.
	Message string `json:"message,omitempty"`
}

// Container is an init or application container within a pod.
//
// The Tilt API representation mirrors the Kubernetes API very closely. Irrelevant data is
// not included, and some fields might be simplified.
//
// There might also be Tilt-specific status fields.
type Container struct {
	// Name is the name of the container as defined in Kubernetes.
	Name string `json:"name"`
	// ID is the normalized container ID (the `docker://` prefix is stripped).
	ID string `json:"id"`
	// Ready is true if the container is passing readiness checks (or has none defined).
	Ready bool `json:"ready"`
	// Image is the image the container is running.
	Image string `json:"image"`
	// Restarts is the number of times the container has restarted.
	//
	// This includes restarts before the Tilt daemon was started if the container was already running.
	Restarts int32 `json:"restarts"`
	// State provides details about the container's current condition.
	State ContainerState `json:"state"`
	// Ports are exposed ports as extracted from the Pod spec.
	//
	// This is added by Tilt for convenience when managing port forwards.
	Ports []int32 `json:"ports"`
}

// ContainerState holds a possible state of container.
//
// Only one of its members may be specified.
// If none of them is specified, the default one is ContainerStateWaiting.
type ContainerState struct {
	// Waiting provides details about a container that is not yet running.
	Waiting *ContainerStateWaiting `json:"waiting"`
	// Running provides details about a currently executing container.
	Running *ContainerStateRunning `json:"running"`
	// Terminated provides details about an exited container.
	Terminated *ContainerStateTerminated `json:"terminated"`
}

// ContainerStateWaiting is a waiting state of a container.
type ContainerStateWaiting struct {
	// Reason is a (brief) reason the container is not yet running.
	Reason string `json:"reason"`
}

// ContainerStateRunning is a running state of a container.
type ContainerStateRunning struct {
	// StartedAt is the time the container began running.
	StartedAt metav1.Time `json:"startedAt"`
}

// ContainerStateTerminated is a terminated state of a container.
type ContainerStateTerminated struct {
	// StartedAt is the time the container began running.
	StartedAt metav1.Time `json:"startedAt"`
	// FinishedAt is the time the container stopped running.
	FinishedAt metav1.Time `json:"finishedAt"`
	// Reason is a (brief) reason the container stopped running.
	Reason string `json:"reason,omitempty"`
	// ExitCode is the exit status from the termination of the container.
	//
	// Any non-zero value indicates an error during termination.
	ExitCode int32 `json:"exitCode"`
}
