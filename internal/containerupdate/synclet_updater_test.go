package containerupdate

import (
	"context"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/sliceutils"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"

	"github.com/tilt-dev/tilt/internal/docker"

	"github.com/tilt-dev/tilt/internal/synclet"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/testutils"
)

func TestUpdateContainer(t *testing.T) {
	f := newSyncletFixture(t)
	defer f.TearDown()

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
	t testing.TB
	*tempdir.TempDirFixture
	ctx    context.Context
	cancel func()
	sm     SyncletManager
	sCli   *synclet.TestSyncletClient
	dCli   *docker.FakeClient
	scu    *SyncletUpdater
}

func newSyncletFixture(t testing.TB) *syncletUpdaterFixture {
	f := tempdir.NewTempDirFixture(t)
	kCli := k8s.NewFakeK8sClient()
	dCli := docker.NewFakeClient()
	sCli := synclet.NewTestSyncletClient(dCli)
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)
	sGRPCCli, err := synclet.FakeGRPCWrapper(ctx, sCli)
	assert.NoError(t, err)
	sm := NewSyncletManagerForTests(kCli, sGRPCCli, sCli)

	cu := &SyncletUpdater{
		sm: sm,
	}

	return &syncletUpdaterFixture{
		t:              t,
		TempDirFixture: f,
		ctx:            ctx,
		cancel:         cancel,
		sm:             sm,
		sCli:           sCli,
		dCli:           dCli,
		scu:            cu,
	}
}

func (f *syncletUpdaterFixture) TearDown() {
	f.cancel()
	f.TempDirFixture.TearDown()
}
