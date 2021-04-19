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
