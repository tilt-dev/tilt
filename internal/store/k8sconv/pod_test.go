package k8sconv

import (
	"fmt"
	"testing"

	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/stretchr/testify/assert"
)

func TestContainerStatusToRuntimeState(t *testing.T) {
	cases := []struct {
		Name   string
		Status v1alpha1.Container
		Result v1alpha1.RuntimeStatus
	}{
		{
			"ok-running", v1alpha1.Container{
				State: v1alpha1.ContainerState{Running: &v1alpha1.ContainerStateRunning{}},
			}, v1alpha1.RuntimeStatusOK,
		},
		{
			"ok-terminated", v1alpha1.Container{
				State: v1alpha1.ContainerState{Terminated: &v1alpha1.ContainerStateTerminated{}},
			}, v1alpha1.RuntimeStatusOK,
		},
		{
			"error-terminated", v1alpha1.Container{
				State: v1alpha1.ContainerState{Terminated: &v1alpha1.ContainerStateTerminated{ExitCode: 1}},
			}, v1alpha1.RuntimeStatusError,
		},
		{
			"error-waiting", v1alpha1.Container{
				State: v1alpha1.ContainerState{Waiting: &v1alpha1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
			}, v1alpha1.RuntimeStatusError,
		},
		{
			"pending-waiting", v1alpha1.Container{
				State: v1alpha1.ContainerState{Waiting: &v1alpha1.ContainerStateWaiting{Reason: "Initializing"}},
			}, v1alpha1.RuntimeStatusPending,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			assert.Equal(t, c.Result, ContainerStatusToRuntimeState(c.Status))
		})
	}
}

func TestPodStatus(t *testing.T) {
	type tc struct {
		pod      v1.PodStatus
		status   string
		messages []string
	}

	cases := []tc{
		{
			pod: v1.PodStatus{
				ContainerStatuses: []v1.ContainerStatus{
					{
						LastTerminationState: v1.ContainerState{
							Terminated: &v1.ContainerStateTerminated{
								ExitCode: 128,
								Message:  "failed to create containerd task: OCI runtime create failed: container_linux.go:345: starting container process caused \"exec: \\\"/hello\\\": stat /hello: no such file or directory\": unknown",
								Reason:   "StartError",
							},
						},
						Ready: false,
						State: v1.ContainerState{
							Waiting: &v1.ContainerStateWaiting{
								Message: "Back-off 40s restarting failed container=my-app pod=my-app-7bb79c789d-8h6n9_default(31369f71-df65-4352-b6bd-6d704a862699)",
								Reason:  "CrashLoopBackOff",
							},
						},
					},
				},
			},
			status: "CrashLoopBackOff",
			messages: []string{
				"failed to create containerd task: OCI runtime create failed: container_linux.go:345: starting container process caused \"exec: \\\"/hello\\\": stat /hello: no such file or directory\": unknown",
				"Back-off 40s restarting failed container=my-app pod=my-app-7bb79c789d-8h6n9_default(31369f71-df65-4352-b6bd-6d704a862699)",
			},
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("case%d", i), func(t *testing.T) {
			pod := v1.Pod{Status: c.pod}
			status := PodStatusToString(pod)
			assert.Equal(t, c.status, status)

			messages := PodStatusErrorMessages(pod)
			assert.Equal(t, c.messages, messages)
		})
	}
}
