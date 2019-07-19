package containerupdate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/synclet"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils"
)

func TestSyncletUpdater_ValidateSpecs(t *testing.T) {
	f := newSyncletFixture(t)

	// iTarg := model.ImageTarget{}
	// k8sTarg := model.K8sTarget{}
	// dcTarg := model.DockerComposeTarget{}

	for _, test := range []struct {
		name        string
		specs       []model.TargetSpec
		env         k8s.Env
		expectedErr error
	}{
		// {"can't update docker compose",
		// 	[]model.TargetSpec{dcTarg},
		// 	k8s.EnvGKE,
		// 	false,
		// },
		// {"can update k8s",
		// 	[]model.TargetSpec{iTarg, k8sTarg},
		// 	k8s.EnvDockerDesktop,
		// 	true,
		// },
	} {
		t.Run(string(test.name), func(t *testing.T) {
			actualErr := f.scu.ValidateSpecs(test.specs, test.env)
			assert.Equal(t, test.expectedErr, actualErr)

		})
	}
}

type syncletUpdaterFixture struct {
	t   testing.TB
	ctx context.Context
	sm  SyncletManager
	scu *SyncletUpdater
}

func newSyncletFixture(t testing.TB) *syncletUpdaterFixture {
	kCli := k8s.NewFakeK8sClient()
	sCli := synclet.NewFakeSyncletClient()
	sm := NewSyncletManagerForTests(kCli, sCli)

	cu := &SyncletUpdater{
		sm: sm,
	}
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()

	return &syncletUpdaterFixture{
		t:   t,
		ctx: ctx,
		sm:  sm,
		scu: cu,
	}
}
