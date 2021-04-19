package k8sconv

import (
	"testing"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/pkg/model"
)

func TestContainerStatusToRuntimeState(t *testing.T) {
	cases := []struct {
		Name   string
		Status v1alpha1.Container
		Result model.RuntimeStatus
	}{
		{
			"ok-running", v1alpha1.Container{
				State: v1alpha1.ContainerState{Running: &v1alpha1.ContainerStateRunning{}},
			}, model.RuntimeStatusOK,
		},
		{
			"ok-terminated", v1alpha1.Container{
				State: v1alpha1.ContainerState{Terminated: &v1alpha1.ContainerStateTerminated{}},
			}, model.RuntimeStatusOK,
		},
		{
			"error-terminated", v1alpha1.Container{
				State: v1alpha1.ContainerState{Terminated: &v1alpha1.ContainerStateTerminated{ExitCode: 1}},
			}, model.RuntimeStatusError,
		},
		{
			"error-waiting", v1alpha1.Container{
				State: v1alpha1.ContainerState{Waiting: &v1alpha1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
			}, model.RuntimeStatusError,
		},
		{
			"pending-waiting", v1alpha1.Container{
				State: v1alpha1.ContainerState{Waiting: &v1alpha1.ContainerStateWaiting{Reason: "Initializing"}},
			}, model.RuntimeStatusPending,
		},
	}

	for _, c := range cases {
		t.Run(c.Name, func(t *testing.T) {
			assert.Equal(t, c.Result, ContainerStatusToRuntimeState(c.Status))
		})
	}
}
