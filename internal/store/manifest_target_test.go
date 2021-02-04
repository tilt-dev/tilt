package store

import (
	"encoding/base64"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/pkg/model"
)

func TestManifestTarget_FacetsSecretsScrubbed(t *testing.T) {
	m := model.Manifest{Name: "test_manifest"}.WithDeployTarget(model.K8sTarget{})
	mt := NewManifestTarget(m)

	s := "password1"
	b64 := base64.StdEncoding.EncodeToString([]byte(s))
	mt.State.BuildStatuses[m.DeployTarget.ID()] = &BuildStatus{
		LastResult: K8sBuildResult{AppliedEntitiesText: fmt.Sprintf("text %s moretext", b64)},
	}
	secrets := model.SecretSet{}
	secrets.AddSecret("foo", "password", []byte(s))
	actual := mt.Facets(secrets)
	expected := []model.Facet{
		{
			Name:  "applied yaml",
			Value: "text [redacted secret foo:password] moretext",
		},
	}

	require.Equal(t, expected, actual)
}

func TestLocalTargetUpdateStatus(t *testing.T) {
	m := model.Manifest{Name: "serve-cmd"}.WithDeployTarget(
		model.NewLocalTarget("serve-cmd", model.Cmd{}, model.ToHostCmd("busybox httpd"), nil))
	mt := NewManifestTarget(m)
	assert.Equal(t, model.UpdateStatusPending, mt.UpdateStatus())

	mt.State.AddCompletedBuild(model.BuildRecord{StartTime: time.Now(), FinishTime: time.Now()})
	assert.Equal(t, model.UpdateStatusNotApplicable, mt.UpdateStatus())

	mt.State.TriggerReason = model.BuildReasonFlagTriggerWeb
	assert.Equal(t, model.UpdateStatusPending, mt.UpdateStatus())
}
