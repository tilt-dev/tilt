package cloud

import (
	"context"
	"fmt"
	"io/ioutil"
	"testing"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/cloud/cloudurl"
	"github.com/windmilleng/tilt/internal/feature"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/testutils/httptest"
	"github.com/windmilleng/tilt/internal/testutils/manifestbuilder"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/internal/token"
	"github.com/windmilleng/tilt/pkg/model"
	proto_webview "github.com/windmilleng/tilt/pkg/webview"
)

func TestOneUpdate(t *testing.T) {
	f := newUpdateFixture(t)
	defer f.TearDown()

	assert.Equal(t, 0, len(f.uu.makeUpdates(f.ctx, f.store).updates()))

	f.AddCompletedBuild("sancho", nil)
	task := f.uu.makeUpdates(f.ctx, f.store)
	assert.Equal(t, 1, len(task.updates()))
	assert.Equal(t, 0, len(f.uu.makeUpdates(f.ctx, f.store).updates()))

	f.uu.sendUpdates(f.ctx, task)
	requests := f.httpClient.Requests
	if assert.Equal(t, 1, len(requests)) {
		body, err := ioutil.ReadAll(requests[0].Body)
		assert.NoError(t, err)
		expected := `{"team_id":{"id":"fake-team"},"updates":[{"service":{"name":"sancho"},"start_time":"1984-04-04T00:00:00Z","duration":"1m0s","is_live_update":false,"result":0,"result_description":"","snapshot_id":{"id":""}}]}
`
		assert.Equal(t, expected, string(body))
	}
}

func TestTiltfileUpdate(t *testing.T) {
	f := newUpdateFixture(t)
	defer f.TearDown()

	assert.Equal(t, 0, len(f.uu.makeUpdates(f.ctx, f.store).updates()))

	f.AddCompletedBuild(store.TiltfileManifestName, nil)
	task := f.uu.makeUpdates(f.ctx, f.store)
	assert.Equal(t, 1, len(task.updates()))
	assert.Equal(t, 0, len(f.uu.makeUpdates(f.ctx, f.store).updates()))

	f.uu.sendUpdates(f.ctx, task)
	requests := f.httpClient.Requests
	if assert.Equal(t, 1, len(requests)) {
		body, err := ioutil.ReadAll(requests[0].Body)
		assert.NoError(t, err)
		expected := `{"team_id":{"id":"fake-team"},"updates":[{"service":{"name":"(Tiltfile)"},"start_time":"1984-04-04T00:00:00Z","duration":"1m0s","is_live_update":false,"result":0,"result_description":"","snapshot_id":{"id":""}}]}
`
		assert.Equal(t, expected, string(body))
	}
}

func TestTwoUpdates(t *testing.T) {
	f := newUpdateFixture(t)
	defer f.TearDown()

	assert.Equal(t, 0, len(f.uu.makeUpdates(f.ctx, f.store).updates()))

	f.AddCompletedBuild("sancho", nil)
	f.AddCompletedBuild("sancho", nil)
	f.AddCompletedBuild("blorg", nil)
	assert.Equal(t, 3, len(f.uu.makeUpdates(f.ctx, f.store).updates()))
	assert.Equal(t, 0, len(f.uu.makeUpdates(f.ctx, f.store).updates()))
}

func TestWatermark(t *testing.T) {
	f := newUpdateFixture(t)
	defer f.TearDown()

	assert.Equal(t, 0, len(f.uu.makeUpdates(f.ctx, f.store).updates()))

	f.AddCompletedBuild("sancho", nil)
	f.AddCompletedBuild("sancho", nil)
	assert.Equal(t, 2, len(f.uu.makeUpdates(f.ctx, f.store).updates()))

	f.AddCompletedBuild("blorg", nil)
	assert.Equal(t, 1, len(f.uu.makeUpdates(f.ctx, f.store).updates()))
}

type updateFixture struct {
	*tempdir.TempDirFixture
	ctx        context.Context
	httpClient *httptest.FakeClient
	uu         *UpdateUploader
	store      *store.Store
	clock      clockwork.FakeClock
}

func newUpdateFixture(t *testing.T) *updateFixture {
	f := tempdir.NewTempDirFixture(t)
	httpClient := httptest.NewFakeClient()
	addr := cloudurl.Address("cloud-test.tilt.dev")
	uu := NewUpdateUploader(httpClient, addr)
	st, _ := store.NewStoreForTesting()
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()

	state := st.LockMutableStateForTesting()
	defer st.UnlockMutableState()

	state.Features = map[string]bool{feature.UpdateHistory: true}
	state.Token = "fake-token"
	state.TeamName = "fake-team"
	state.TiltCloudUsername = "fake-username"

	m1 := manifestbuilder.New(f, "sancho").WithK8sYAML(testyaml.SanchoYAML).Build()
	state.UpsertManifestTarget(store.NewManifestTarget(m1))

	m2 := manifestbuilder.New(f, "blorg").WithK8sYAML(testyaml.BlorgBackendYAML).Build()
	state.UpsertManifestTarget(store.NewManifestTarget(m2))

	return &updateFixture{
		TempDirFixture: f,
		ctx:            ctx,
		httpClient:     httpClient,
		uu:             uu,
		store:          st,
		clock:          clockwork.NewFakeClock(),
	}
}

func (f *updateFixture) AddCompletedBuild(name model.ManifestName, err error) {
	state := f.store.LockMutableStateForTesting()
	defer f.store.UnlockMutableState()

	record := model.BuildRecord{
		StartTime:  f.clock.Now(),
		FinishTime: f.clock.Now().Add(time.Minute),
		Error:      err,
	}
	var ms *store.ManifestState
	if name == store.TiltfileManifestName {
		ms = &state.TiltfileState
	} else {
		var ok bool
		ms, ok = state.ManifestState(name)
		if !ok {
			panic(fmt.Errorf("no manifest with name %s", name))
		}
	}

	ms.AddCompletedBuild(record)
	state.CompletedBuildCount++
	f.clock.Advance(2 * time.Minute)
}

type fakeSnapshotUploader struct {
	count int
}

var _ SnapshotUploader = &fakeSnapshotUploader{}

func (f *fakeSnapshotUploader) TakeAndUpload(state store.EngineState) (SnapshotID, error) {
	f.count++
	return SnapshotID(fmt.Sprintf("snapshot%d", f.count)), nil
}

func (f *fakeSnapshotUploader) Upload(token token.Token, teamID string, snapshot *proto_webview.Snapshot) (SnapshotID, error) {
	panic("not implemented")
}

func (f *fakeSnapshotUploader) IDToSnapshotURL(id SnapshotID) string {
	panic("not implemented")
}
