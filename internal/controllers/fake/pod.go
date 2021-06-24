package fake

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s"
)

type PodBuilder corev1.Pod

type PodBuilderOption func(builder *PodBuilder)

func NewPodBuilder(id k8s.PodID, opts ...PodBuilderOption) *PodBuilder {
	return (*PodBuilder)(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:       types.UID(id + "-uid"),
			Name:      string(id),
			Namespace: "default",
		},
	})
}

func WithNamespace(ns k8s.Namespace) PodBuilderOption {
	return func(builder *PodBuilder) {
		builder.Namespace = string(ns)
	}
}

func WithLabels(labels map[string]string) PodBuilderOption {
	return func(builder *PodBuilder) {
		for k, v := range labels {
			builder.Labels[k] = v
		}
	}
}

func (pb *PodBuilder) AddRunningContainer(name container.Name, id container.ID) *PodBuilder {
	pb.Spec.Containers = append(pb.Spec.Containers, corev1.Container{
		Name: string(name),
	})
	pb.Status.ContainerStatuses = append(pb.Status.ContainerStatuses, corev1.ContainerStatus{
		Name:        string(name),
		ContainerID: fmt.Sprintf("containerd://%s", id),
		Image:       fmt.Sprintf("image-%s", strings.ToLower(string(name))),
		ImageID:     fmt.Sprintf("image-%s", strings.ToLower(string(name))),
		Ready:       true,
		State: corev1.ContainerState{
			Running: &corev1.ContainerStateRunning{
				StartedAt: metav1.Now(),
			},
		},
	})
	return pb
}

func (pb *PodBuilder) AddRunningInitContainer(name container.Name, id container.ID) *PodBuilder {
	pb.Spec.InitContainers = append(pb.Spec.InitContainers, corev1.Container{
		Name: string(name),
	})
	pb.Status.InitContainerStatuses = append(pb.Status.InitContainerStatuses, corev1.ContainerStatus{
		Name:        string(name),
		ContainerID: fmt.Sprintf("containerd://%s", id),
		Image:       fmt.Sprintf("image-%s", strings.ToLower(string(name))),
		ImageID:     fmt.Sprintf("image-%s", strings.ToLower(string(name))),
		Ready:       true,
		State: corev1.ContainerState{
			Running: &corev1.ContainerStateRunning{
				StartedAt: metav1.Now(),
			},
		},
	})
	return pb
}

func (pb *PodBuilder) AddTerminatedContainer(name container.Name, id container.ID) *PodBuilder {
	pb.AddRunningContainer(name, id)
	statuses := pb.Status.ContainerStatuses
	statuses[len(statuses)-1].State.Running = nil
	statuses[len(statuses)-1].State.Terminated = &corev1.ContainerStateTerminated{
		StartedAt: metav1.Now(),
	}
	return pb
}

func (pb *PodBuilder) AddTerminatedInitContainer(name container.Name, id container.ID) *PodBuilder {
	pb.AddRunningInitContainer(name, id)
	statuses := pb.Status.InitContainerStatuses
	statuses[len(statuses)-1].State.Running = nil
	statuses[len(statuses)-1].State.Terminated = &corev1.ContainerStateTerminated{
		StartedAt: metav1.Now(),
	}
	return pb
}

func (pb *PodBuilder) ToPod() *corev1.Pod {
	return (*corev1.Pod)(pb)
}
