package containerupdate

import (
	"context"
	"testing"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/testutils"
)

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
