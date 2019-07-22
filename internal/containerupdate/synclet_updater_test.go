package containerupdate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/synclet"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/testutils"
)

func TestUpdateContainer(t *testing.T) {
	f := newSyncletFixture(t)

	err := f.scu.UpdateContainer(f.ctx, TestDeployInfo, newReader("hello world"), toDelete, cmds, false)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 1, f.sCli.UpdateContainerCount)
	assert.Equal(t, []byte("hello world"), f.sCli.LastTarArchiveBytes)
	assert.Equal(t, toDelete, f.sCli.LastFilesToDelete)
	assert.Equal(t, 2, f.sCli.CommandsRunCount)
	assert.False(t, f.sCli.LastHotReload)
}

type syncletUpdaterFixture struct {
	t    testing.TB
	ctx  context.Context
	sm   SyncletManager
	sCli *synclet.FakeSyncletClient
	scu  *SyncletUpdater
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
		t:    t,
		ctx:  ctx,
		sm:   sm,
		sCli: sCli,
		scu:  cu,
	}
}
