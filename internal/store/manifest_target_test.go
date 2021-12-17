package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestLocalTargetUpdateStatus(t *testing.T) {
	m := model.Manifest{Name: "serve-cmd"}.WithDeployTarget(
		model.NewLocalTarget("serve-cmd", model.Cmd{}, model.ToHostCmd("busybox httpd"), nil))
	mt := NewManifestTarget(m)
	assert.Equal(t, v1alpha1.UpdateStatusPending, mt.UpdateStatus())
	assert.Equal(t, v1alpha1.RuntimeStatusPending, mt.RuntimeStatus())

	mt.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}
	assert.Equal(t, v1alpha1.UpdateStatusPending, mt.UpdateStatus())
	assert.Equal(t, v1alpha1.RuntimeStatusPending, mt.RuntimeStatus())

	mt.State.CurrentBuild = model.BuildRecord{}
	mt.State.AddCompletedBuild(model.BuildRecord{StartTime: time.Now(), FinishTime: time.Now()})
	assert.Equal(t, v1alpha1.UpdateStatusNotApplicable, mt.UpdateStatus())

	// We currently have an unknown runtime state when the build completes,
	// but we haven't received any data from the runtime yet.
	assert.Equal(t, v1alpha1.RuntimeStatusUnknown, mt.RuntimeStatus())

	mt.State.TriggerReason = model.BuildReasonFlagTriggerWeb
	assert.Equal(t, v1alpha1.UpdateStatusPending, mt.UpdateStatus())
	assert.Equal(t, v1alpha1.RuntimeStatusPending, mt.RuntimeStatus())
}

func TestK8sRuntimeStatus(t *testing.T) {
	m := model.Manifest{Name: "k8s"}.WithDeployTarget(model.NewK8sTargetForTesting(""))
	mt := NewManifestTarget(m)
	assert.Equal(t, v1alpha1.UpdateStatusPending, mt.UpdateStatus())
	assert.Equal(t, v1alpha1.RuntimeStatusPending, mt.RuntimeStatus())

	mt.Manifest.TriggerMode = model.TriggerModeManual
	assert.Equal(t, v1alpha1.UpdateStatusNone, mt.UpdateStatus())
	assert.Equal(t, v1alpha1.RuntimeStatusNone, mt.RuntimeStatus())
}
