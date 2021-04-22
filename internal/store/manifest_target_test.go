package store

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/pkg/model"
)

func TestLocalTargetUpdateStatus(t *testing.T) {
	m := model.Manifest{Name: "serve-cmd"}.WithDeployTarget(
		model.NewLocalTarget("serve-cmd", model.Cmd{}, model.ToHostCmd("busybox httpd"), nil))
	mt := NewManifestTarget(m)
	assert.Equal(t, model.UpdateStatusPending, mt.UpdateStatus())

	mt.State.CurrentBuild = model.BuildRecord{StartTime: time.Now()}
	assert.Equal(t, model.UpdateStatusPending, mt.UpdateStatus())

	mt.State.CurrentBuild = model.BuildRecord{}
	mt.State.AddCompletedBuild(model.BuildRecord{StartTime: time.Now(), FinishTime: time.Now()})
	assert.Equal(t, model.UpdateStatusNotApplicable, mt.UpdateStatus())

	mt.State.TriggerReason = model.BuildReasonFlagTriggerWeb
	assert.Equal(t, model.UpdateStatusPending, mt.UpdateStatus())
}
