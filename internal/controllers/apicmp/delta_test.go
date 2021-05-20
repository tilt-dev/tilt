package apicmp

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func now() time.Time {
	return time.Unix(1619635910, 450240689)
}

func TestCmp(t *testing.T) {
	cmd := &v1alpha1.Cmd{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "cmd",
			CreationTimestamp: metav1.NewTime(now()),
		},
		Status: v1alpha1.CmdStatus{
			Running: &v1alpha1.CmdStateRunning{
				StartedAt: metav1.NewMicroTime(now()),
			},
		},
	}

	assert.True(t, DeepEqual(cmd, &v1alpha1.Cmd{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "cmd",
			CreationTimestamp: metav1.NewTime(now().Add(time.Nanosecond)),
		},
		Status: v1alpha1.CmdStatus{
			Running: &v1alpha1.CmdStateRunning{
				StartedAt: metav1.NewMicroTime(now().Add(time.Nanosecond)),
			},
		},
	}))
	assert.True(t, DeepEqual(cmd, &v1alpha1.Cmd{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "cmd",
			CreationTimestamp: metav1.NewTime(now().Add(time.Microsecond)),
		},
		Status: v1alpha1.CmdStatus{
			Running: &v1alpha1.CmdStateRunning{
				StartedAt: metav1.NewMicroTime(now().Add(time.Nanosecond)),
			},
		},
	}))
	assert.False(t, DeepEqual(cmd, &v1alpha1.Cmd{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "cmd",
			CreationTimestamp: metav1.NewTime(now().Add(time.Second)),
		},
		Status: v1alpha1.CmdStatus{
			Running: &v1alpha1.CmdStateRunning{
				StartedAt: metav1.NewMicroTime(now()),
			},
		},
	}))
	assert.False(t, DeepEqual(cmd, &v1alpha1.Cmd{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "cmd",
			CreationTimestamp: metav1.NewTime(now()),
		},
		Status: v1alpha1.CmdStatus{
			Running: &v1alpha1.CmdStateRunning{
				StartedAt: metav1.NewMicroTime(now().Add(time.Microsecond)),
			},
		},
	}))

}
