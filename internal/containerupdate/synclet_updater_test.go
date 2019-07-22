package containerupdate

import (
	"context"
	"testing"

	"github.com/windmilleng/tilt/internal/synclet"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/testutils"
)

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
