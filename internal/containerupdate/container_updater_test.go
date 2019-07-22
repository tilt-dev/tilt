package containerupdate

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/model"
)

func TestSupportsSpecsOnlyImageDeployedToK8s(t *testing.T) {
	iTarg := model.ImageTarget{}
	k8sTargWithDep := model.K8sTarget{}.WithDependencyIDs([]model.TargetID{iTarg.ID()})
	k8sTargNoDep := model.K8sTarget{}
	dcTarg := model.DockerComposeTarget{}

	for _, test := range []struct {
		name        string
		specs       []model.TargetSpec
		expectedMsg string
	}{
		{"can't update docker compose",
			[]model.TargetSpec{dcTarg},
			"MyUpdater does not support DockerCompose targets (this should never happen: please contact Tilt support)",
		},
		{"can update image target deployed to k8s",
			[]model.TargetSpec{iTarg, k8sTargWithDep},
			"",
		},
		{"can't update image target NOT deployed to k8s",
			[]model.TargetSpec{iTarg, k8sTargNoDep},
			"MyUpdater can only handle images deployed to k8s (i.e. not base images)",
		},
		{"local cluster ok",
			[]model.TargetSpec{iTarg, k8sTargWithDep},
			"",
		},
	} {
		t.Run(string(test.name), func(t *testing.T) {
			ok, msg := specsAreOnlyImagesDeployedToK8s(test.specs, "MyUpdater")
			expectOk := test.expectedMsg == ""
			assert.Equal(t, expectOk, ok, "expected ok = %t but got %t", expectOk, ok)
			assert.Equal(t, test.expectedMsg, msg)
		})
	}
}
