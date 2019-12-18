package k8s

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/container"
)

func TestFixContainerStatusImages(t *testing.T) {
	pod := fakePod(expectedPod, blorgDevImgStr)
	pod.Status = v1.PodStatus{
		ContainerStatuses: []v1.ContainerStatus{
			{
				Name:  "default",
				Image: blorgDevImgStr + "v2",
				Ready: true,
			},
		},
	}

	assert.NotEqual(t,
		pod.Spec.Containers[0].Image,
		pod.Status.ContainerStatuses[0].Image)
	FixContainerStatusImages(pod)
	assert.Equal(t,
		pod.Spec.Containers[0].Image,
		pod.Status.ContainerStatuses[0].Image)
}

func TestWaitForContainerAlreadyAlive(t *testing.T) {
	f := newClientTestFixture(t)

	nt := container.MustParseSelector(blorgDevImgStr)
	podData := fakePod(expectedPod, blorgDevImgStr)
	podData.Status = v1.PodStatus{
		ContainerStatuses: []v1.ContainerStatus{
			{
				ContainerID: "docker://container-id",
				Image:       nt.String(),
				Ready:       true,
			},
		},
	}
	f.addObject(podData)

	ctx, cancel := context.WithTimeout(f.ctx, time.Second)
	defer cancel()

	pod := f.getPod(expectedPod)
	cStatus, err := WaitForContainerReady(ctx, f.client, pod, nt)
	if err != nil {
		t.Fatal(err)
	}

	cID, err := ContainerIDFromContainerStatus(cStatus)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "container-id", cID.String())
}

func TestWaitForContainerSuccess(t *testing.T) {
	f := newClientTestFixture(t)
	f.addObject(&fakePodList)

	nt := container.MustParseTaggedSelector(blorgDevImgStr)
	pod := f.getPod(expectedPod)

	ctx, cancel := context.WithTimeout(f.ctx, time.Second)
	defer cancel()

	result := make(chan error)
	go func() {
		_, err := WaitForContainerReady(ctx, f.client, pod, nt)
		result <- err
	}()

	newPod := fakePod(expectedPod, blorgDevImgStr)
	newPod.Status = v1.PodStatus{
		ContainerStatuses: []v1.ContainerStatus{
			{
				ContainerID: "docker://container-id",
				Image:       nt.String(),
				Ready:       true,
			},
		},
	}

	<-f.watchNotify
	f.updatePod(newPod)
	err := <-result
	if err != nil {
		t.Fatal(err)
	}
}

func TestWaitForContainerFailure(t *testing.T) {
	f := newClientTestFixture(t)
	f.addObject(&fakePodList)

	nt := container.MustParseTaggedSelector(blorgDevImgStr)
	pod := f.getPod(expectedPod)

	ctx, cancel := context.WithTimeout(f.ctx, time.Second)
	defer cancel()

	result := make(chan error)
	go func() {
		_, err := WaitForContainerReady(ctx, f.client, pod, nt)
		result <- err
	}()

	newPod := fakePod(expectedPod, blorgDevImgStr)
	newPod.Status = v1.PodStatus{
		ContainerStatuses: []v1.ContainerStatus{
			{
				Image: nt.String(),
				State: v1.ContainerState{
					Terminated: &v1.ContainerStateTerminated{},
				},
			},
		},
	}

	<-f.watchNotify
	f.updatePod(newPod)
	err := <-result

	expected := "Container will never be ready"
	if err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("Expected error %q, actual: %v", expected, err)
	}
}

func TestWaitForContainerUnschedulable(t *testing.T) {
	f := newClientTestFixture(t)
	f.addObject(&fakePodList)

	nt := container.MustParseTaggedSelector(blorgDevImgStr)
	pod := f.getPod(expectedPod)

	ctx, cancel := context.WithTimeout(f.ctx, time.Second)
	defer cancel()

	result := make(chan error)
	go func() {
		_, err := WaitForContainerReady(ctx, f.client, pod, nt)
		result <- err
	}()

	newPod := fakePod(expectedPod, blorgDevImgStr)
	newPod.Status = v1.PodStatus{
		Conditions: []v1.PodCondition{
			{
				Reason:  v1.PodReasonUnschedulable,
				Message: "0/4 nodes are available: 4 Insufficient cpu.",
				Status:  "False",
				Type:    v1.PodScheduled,
			},
		},
	}

	<-f.watchNotify
	f.updatePod(newPod)
	err := <-result

	expected := "Container will never be ready: 0/4 nodes are available: 4 Insufficient cpu."
	if err == nil || !strings.Contains(err.Error(), expected) {
		t.Fatalf("Expected error %q, actual: %v", expected, err)
	}
}
