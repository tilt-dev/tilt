package k8sconv

import (
	"testing"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/pkg/model"
)

func TestContainerStatusToRuntimeState(t *testing.T) {
	cases := []struct {
		Name   string
		Status v1.ContainerStatus
		Result model.RuntimeStatus
	}{
		{
			"ok-running", v1.ContainerStatus{
				State: v1.ContainerState{Running: &v1.ContainerStateRunning{}},
			}, model.RuntimeStatusOK,
		},
		{
			"ok-terminated", v1.ContainerStatus{
				State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{}},
			}, model.RuntimeStatusOK,
		},
		{
			"error-terminated", v1.ContainerStatus{
				State: v1.ContainerState{Terminated: &v1.ContainerStateTerminated{ExitCode: 1}},
			}, model.RuntimeStatusError,
		},
		{
			"error-waiting", v1.ContainerStatus{
				State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
			}, model.RuntimeStatusError,
		},
		{
			"pending-waiting", v1.ContainerStatus{
				State: v1.ContainerState{Waiting: &v1.ContainerStateWaiting{Reason: "Initializing"}},
			}, model.RuntimeStatusPending,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			assert.Equal(t, c.Result, ContainerStatusToRuntimeState(c.Status))
		})
	}
}
