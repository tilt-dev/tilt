package store_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestK8sRuntimeState_RuntimeStatus_NeverDeployed(t *testing.T) {
	s := store.K8sRuntimeState{
		HasEverDeployedSuccessfully: false,
		PodReadinessMode:            model.PodReadinessIgnore,
	}
	assert.Equal(t, model.RuntimeStatusPending, s.RuntimeStatus())
}

func TestK8sRuntimeState_RuntimeStatus_PodReadinessIgnore(t *testing.T) {
	s := store.K8sRuntimeState{
		HasEverDeployedSuccessfully: true,
		PodReadinessMode:            model.PodReadinessIgnore,
	}
	assert.Equal(t, model.RuntimeStatusOK, s.RuntimeStatus())
}

func TestK8sRuntimeState_RuntimeStatus(t *testing.T) {
	type tc struct {
		name           string
		expectedStatus model.RuntimeStatus
		pod            store.Pod
	}

	tcs := []tc{
		{
			name:           "Phase_PodRunning_AllContainersReady",
			expectedStatus: model.RuntimeStatusOK,
			pod: store.Pod{
				Phase:      v1.PodRunning,
				Containers: []store.Container{{Ready: true}},
			},
		},
		{
			name:           "Phase_PodRunning_SomeContainersNotReady",
			expectedStatus: model.RuntimeStatusPending,
			pod: store.Pod{
				Phase:      v1.PodRunning,
				Containers: []store.Container{{Ready: true}, {Ready: false}},
			},
		},
		{
			name:           "Phase_PodSucceeded",
			expectedStatus: model.RuntimeStatusOK,
			pod:            store.Pod{Phase: v1.PodSucceeded},
		},
		{
			name:           "Phase_PodFailed",
			expectedStatus: model.RuntimeStatusError,
			pod:            store.Pod{Phase: v1.PodFailed},
		},
		{
			name:           "Conditions_Unschedulable_WithinThreshold",
			expectedStatus: model.RuntimeStatusPending,
			pod: store.Pod{
				Phase: v1.PodPending,
				Conditions: []v1.PodCondition{{
					Type:               v1.PodScheduled,
					Reason:             v1.PodReasonUnschedulable,
					LastTransitionTime: metav1.Now(),
				}},
			},
		},
		{
			name:           "Conditions_Unschedulable_ExceededThreshold",
			expectedStatus: model.RuntimeStatusError,
			pod: store.Pod{
				Phase: v1.PodPending,
				Conditions: []v1.PodCondition{{
					Type:               v1.PodScheduled,
					Reason:             v1.PodReasonUnschedulable,
					LastTransitionTime: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
				}},
			},
		},
		{
			name:           "ContainerStatus_Error",
			expectedStatus: model.RuntimeStatusError,
			pod: store.Pod{
				Containers: []store.Container{{Status: model.RuntimeStatusOK}, {Status: model.RuntimeStatusError}},
			},
		},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			s := store.K8sRuntimeState{
				HasEverDeployedSuccessfully: true,
				PodReadinessMode:            model.PodReadinessWait,
				Pods:                        map[k8s.PodID]*store.Pod{"test": &tc.pod},
			}
			assert.Equal(t, tc.expectedStatus, s.RuntimeStatus())
		})
	}
}
