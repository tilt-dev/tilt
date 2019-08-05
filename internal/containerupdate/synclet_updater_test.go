package containerupdate

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/windmilleng/tilt/internal/sliceutils"

	"github.com/windmilleng/tilt/internal/docker"

	"github.com/windmilleng/tilt/internal/synclet"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/testutils"
)

func TestUpdateContainer(t *testing.T) {
	f := newSyncletFixture(t)

	err := f.scu.UpdateContainer(f.ctx, TestContainerInfo, newReader("hello world"), toDelete, cmds, false)
	if err != nil {
		t.Fatal(err)
	}

	copyContents, err := ioutil.ReadAll(f.dCli.CopyContent)
	require.Nil(t, err)

	assert.Equal(t, 1, f.sCli.UpdateContainerCount)
	assert.Equal(t, []byte("hello world"), copyContents)

	require.Len(t, f.dCli.ExecCalls, 3)
	assert.True(t, sliceutils.StringSliceStartsWith(f.dCli.ExecCalls[0].Cmd.Argv, "rm"))
	assert.True(t, sliceutils.StringSliceStartsWith(f.dCli.ExecCalls[1].Cmd.Argv, "a"))
	assert.True(t, sliceutils.StringSliceStartsWith(f.dCli.ExecCalls[2].Cmd.Argv, "b"))
	assert.Len(t, f.dCli.RestartsByContainer, 1)
}

type syncletUpdaterFixture struct {
	t    testing.TB
	ctx  context.Context
	sm   SyncletManager
	sCli *synclet.TestSyncletClient
	dCli *docker.FakeClient
	scu  *SyncletUpdater
}

func newSyncletFixture(t testing.TB) *syncletUpdaterFixture {
	kCli := k8s.NewFakeK8sClient()
	dCli := docker.NewFakeClient()
	sCli := synclet.NewTestSyncletClient(dCli)
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
		dCli: dCli,
		scu:  cu,
	}
}
