package containerupdate

import (
	"context"
	"os"
	"testing"

	"github.com/windmilleng/tilt/internal/testutils"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/docker"

	"github.com/windmilleng/tilt/internal/testutils/manifestutils"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/synclet"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestSyncletSubscriptions(t *testing.T) {
	f := newSMFixture(t)
	defer f.TearDown()

	state := f.store.LockMutableStateForTesting()
	state.WatchFiles = true
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"},
		store.Pod{
			PodID:      "pod-id",
			HasSynclet: true,
		}))
	f.store.UnlockMutableState()

	f.sm.OnChange(f.ctx, f.store)
	assert.Equal(t, "pod-id", string(f.sCli.PodID))
	assert.Equal(t, 1, len(f.sm.clients))

	state = f.store.LockMutableStateForTesting()
	state.UpsertManifestTarget(manifestutils.NewManifestTargetWithPod(
		model.Manifest{Name: "server"},
		store.Pod{
			PodID: "pod-id",
		}))
	f.store.UnlockMutableState()

	f.sm.OnChange(f.ctx, f.store)
	assert.Equal(t, 0, len(f.sm.clients))
}

type smFixture struct {
	*tempdir.TempDirFixture
	ctx    context.Context
	cancel func()
	kCli   *k8s.FakeK8sClient
	sCli   *synclet.TestSyncletClient
	sm     SyncletManager
	store  *store.Store
}

func newSMFixture(t *testing.T) *smFixture {
	f := tempdir.NewTempDirFixture(t)
	kCli := k8s.NewFakeK8sClient()
	dCli := docker.NewFakeClient()
	sCli := synclet.NewTestSyncletClient(dCli)
	ctx, cancel := context.WithCancel(context.Background())
	sGRPCCli, err := synclet.FakeGRPCWrapper(ctx, sCli)
	assert.NoError(t, err)
	sm := NewSyncletManagerForTests(kCli, sGRPCCli, sCli)
	st, _ := store.NewStoreForTesting()

	l := logger.NewLogger(logger.DebugLvl, os.Stdout)
	ctx = logger.WithLogger(ctx, l)
	go func() {
		err := st.Loop(ctx)
		testutils.FailOnNonCanceledErr(t, err, "store.Loop failed")
	}()

	return &smFixture{
		TempDirFixture: f,
		ctx:            ctx,
		cancel:         cancel,
		store:          st,
		kCli:           kCli,
		sCli:           sCli,
		sm:             sm,
	}
}

func (f *smFixture) TearDown() {
	f.cancel()
	f.kCli.TearDown()
	f.TempDirFixture.TearDown()
}
