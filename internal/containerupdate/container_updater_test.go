package containerupdate

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/engine/errors"
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
		expectedErr error
	}{
		{"can't update docker compose",
			[]model.TargetSpec{dcTarg},
			fmt.Errorf("MyUpdater does not support DockerCompose targets (this should never happen: please contact Tilt support)"),
		},
		{"can update image target deployed to k8s",
			[]model.TargetSpec{iTarg, k8sTargWithDep},
			nil,
		},
		{"can't update image target NOT deployed to k8s",
			[]model.TargetSpec{iTarg, k8sTargNoDep},
			errors.RedirectToNextBuilderInfof("MyUpdater can only handle images deployed to k8s (i.e. not base images)"),
		},
		{"local cluster ok",
			[]model.TargetSpec{iTarg, k8sTargWithDep},
			nil,
		},
	} {
		t.Run(string(test.name), func(t *testing.T) {
			actualErr := validateSpecsOnlyImagesDeployedToK8s(test.specs, "MyUpdater")
			assert.Equal(t, test.expectedErr, actualErr)
		})
	}
}
