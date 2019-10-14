package cloud

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/feature"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/testutils/httptest"
)

func TestAutoUpload(t *testing.T) {
	f := newSUFixture(t)

	f.su.OnChange(f.ctx, f.store)
	assert.Equal(t, 0, len(f.httpClient.Requests))

	f.completeBuild()
	f.su.OnChange(f.ctx, f.store)
	assert.Equal(t, 1, len(f.httpClient.Requests))

	f.su.OnChange(f.ctx, f.store)
	assert.Equal(t, 1, len(f.httpClient.Requests))
}

type snapshotUploaderFixture struct {
	ctx        context.Context
	httpClient *httptest.FakeClient
	su         *SnapshotUploader
	store      *store.Store
}

func newSUFixture(t *testing.T) *snapshotUploaderFixture {
	httpClient := httptest.NewFakeClient()
	addr := Address("cloud-test.tilt.dev")
	su := NewSnapshotUploader(httpClient, addr)
	store, _ := store.NewStoreForTesting()
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()

	state := store.LockMutableStateForTesting()
	state.Features = map[string]bool{feature.SnapshotsAutoUpload: true}
	state.Token = "fake-token"
	defer store.UnlockMutableState()

	return &snapshotUploaderFixture{
		ctx:        ctx,
		httpClient: httpClient,
		su:         su,
		store:      store,
	}
}

func (f *snapshotUploaderFixture) completeBuild() {
	st := f.store.LockMutableStateForTesting()
	defer f.store.UnlockMutableState()

	st.CompletedBuildCount++
}
