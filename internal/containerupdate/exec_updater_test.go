package containerupdate

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/engine/errors"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils"
)

func TestExecUpdater_SupportsSpecs(t *testing.T) {
	f := newExecFixture(t)

	iTarg := model.ImageTarget{}
	k8sTargWithDep := model.K8sTarget{}.WithDependencyIDs([]model.TargetID{iTarg.ID()})
	k8sTargNoDep := model.K8sTarget{}
	dcTarg := model.DockerComposeTarget{}

	for _, test := range []struct {
		name        string
		specs       []model.TargetSpec
		env         k8s.Env
		expectedErr error
	}{
		{"can't update docker compose",
			[]model.TargetSpec{dcTarg},
			k8s.EnvGKE,
			fmt.Errorf("ExecUpdater does not support DockerCompose targets (this should never happen: please contact Tilt support)"),
		},
		{"can update image target deployed to k8s",
			[]model.TargetSpec{iTarg, k8sTargWithDep},
			k8s.EnvGKE,
			nil,
		},
		{"can't update image target NOT deployed to k8s",
			[]model.TargetSpec{iTarg, k8sTargNoDep},
			k8s.EnvGKE,
			errors.RedirectToNextBuilderInfof("ExecUpdater can only handle images deployed to k8s (i.e. not base images)"),
		},
		{"local cluster ok",
			[]model.TargetSpec{iTarg, k8sTargWithDep},
			k8s.EnvMinikube,
			nil,
		},
	} {
		t.Run(string(test.name), func(t *testing.T) {
			actualErr := f.ecu.SupportsSpecs(test.specs)
			assert.Equal(t, test.expectedErr, actualErr)
		})
	}
}

type execUpdaterFixture struct {
	t    testing.TB
	ctx  context.Context
	kCli *k8s.FakeK8sClient
	ecu  *ExecUpdater
}

func newExecFixture(t testing.TB) *execUpdaterFixture {
	fakeCli := k8s.NewFakeK8sClient()
	cu := &ExecUpdater{
		kCli: fakeCli,
	}
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()

	return &execUpdaterFixture{
		t:    t,
		ctx:  ctx,
		kCli: fakeCli,
		ecu:  cu,
	}
}
