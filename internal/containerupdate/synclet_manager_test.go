package containerupdate

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/docker"

	"github.com/tilt-dev/tilt/internal/testutils/manifestutils"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/synclet"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestSyncletSubscriptions(t *testing.T) {
	f := newSMFixture(t)
	defer f.TearDown()

	state := f.store.LockMutableStateForTesting()
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
	store  *store.TestingStore
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
	st := store.NewTestingStore()

	l := logger.NewLogger(logger.DebugLvl, os.Stdout)
	ctx = logger.WithLogger(ctx, l)

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
