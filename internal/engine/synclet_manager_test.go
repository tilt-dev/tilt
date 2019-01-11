package engine

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/synclet"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

func TestSyncletSubscriptions(t *testing.T) {
	f := newSMFixture(t)
	defer f.TearDown()

	state := f.store.LockMutableStateForTesting()
	state.WatchMounts = true
	state.UpsertManifestTarget(newManifestTargetWithPod(
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
	state.UpsertManifestTarget(newManifestTargetWithPod(
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
	sCli   *synclet.FakeSyncletClient
	sm     SyncletManager
	store  *store.Store
}

func newSMFixture(t *testing.T) *smFixture {
	f := tempdir.NewTempDirFixture(t)
	kCli := k8s.NewFakeK8sClient()
	sCli := synclet.NewFakeSyncletClient()
	sm := NewSyncletManagerForTests(kCli, sCli)
	st := store.NewStoreForTesting()

	ctx, cancel := context.WithCancel(context.Background())
	l := logger.NewLogger(logger.DebugLvl, os.Stdout)
	ctx = logger.WithLogger(ctx, l)
	go st.Loop(ctx)

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
	f.TempDirFixture.TearDown()
}
