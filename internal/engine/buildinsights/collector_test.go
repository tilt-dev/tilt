package buildinsights

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
)

// mockStore implements model.BuildInsightsStore for testing.
type mockStore struct {
	mu      sync.Mutex
	metrics []model.BuildMetric
}

func newMockStore() *mockStore {
	return &mockStore{
		metrics: make([]model.BuildMetric, 0),
	}
}

func (m *mockStore) RecordBuild(metric model.BuildMetric) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.metrics = append(m.metrics, metric)
	return nil
}

func (m *mockStore) GetInsights(since time.Time) (*model.BuildInsights, error) {
	return &model.BuildInsights{}, nil
}

func (m *mockStore) GetResourceStats(name model.ManifestName) (*model.ResourceStats, error) {
	return &model.ResourceStats{ManifestName: name}, nil
}

func (m *mockStore) GetRecentBuilds(n int) ([]model.BuildMetric, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if n > len(m.metrics) {
		n = len(m.metrics)
	}
	return m.metrics[:n], nil
}

func (m *mockStore) GetBuildHistory(name model.ManifestName, since time.Time) ([]model.BuildMetric, error) {
	return nil, nil
}

func (m *mockStore) ClearOlderThan(t time.Time) error {
	return nil
}

func (m *mockStore) Close() error {
	return nil
}

func (m *mockStore) getMetrics() []model.BuildMetric {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]model.BuildMetric, len(m.metrics))
	copy(result, m.metrics)
	return result
}

// fakeRStore implements store.RStore for testing.
type fakeRStore struct {
	state    store.EngineState
	stateMu  sync.RWMutex
	unlocked bool
}

func newFakeRStore() *fakeRStore {
	return &fakeRStore{
		state: *store.NewState(),
	}
}

func (f *fakeRStore) Dispatch(action store.Action) {}

func (f *fakeRStore) RLockState() store.EngineState {
	f.stateMu.RLock()
	return f.state
}

func (f *fakeRStore) RUnlockState() {
	f.stateMu.RUnlock()
}

func (f *fakeRStore) StateMutex() *sync.RWMutex {
	return &f.stateMu
}

func (f *fakeRStore) SetManifestTarget(mt *store.ManifestTarget) {
	f.stateMu.Lock()
	defer f.stateMu.Unlock()
	if f.state.ManifestTargets == nil {
		f.state.ManifestTargets = make(map[model.ManifestName]*store.ManifestTarget)
	}
	f.state.ManifestTargets[mt.Manifest.Name] = mt
}

func TestCollector_OnChange_RecordsBuild(t *testing.T) {
	mockSt := newMockStore()
	collector := NewCollector(mockSt)

	rs := newFakeRStore()

	// Create a manifest with a completed build
	m := model.Manifest{Name: "test-service"}
	mt := store.NewManifestTarget(m)
	mt.State.BuildHistory = []model.BuildRecord{
		{
			StartTime:    time.Now().Add(-10 * time.Second),
			FinishTime:   time.Now(),
			Reason:       model.BuildReasonFlagChangedFiles,
			Error:        nil,
			WarningCount: 0,
			BuildTypes:   []model.BuildType{model.BuildTypeImage},
		},
	}
	rs.SetManifestTarget(mt)

	// Call OnChange
	ctx := context.Background()
	err := collector.OnChange(ctx, rs, store.ChangeSummary{})
	require.NoError(t, err)

	// Verify metric was recorded
	metrics := mockSt.getMetrics()
	require.Len(t, metrics, 1)
	assert.Equal(t, model.ManifestName("test-service"), metrics[0].ManifestName)
	assert.True(t, metrics[0].Success)
}

func TestCollector_OnChange_SkipsDuplicates(t *testing.T) {
	mockSt := newMockStore()
	collector := NewCollector(mockSt)

	rs := newFakeRStore()

	finishTime := time.Now()

	// Create a manifest with a completed build
	m := model.Manifest{Name: "test-service"}
	mt := store.NewManifestTarget(m)
	mt.State.BuildHistory = []model.BuildRecord{
		{
			StartTime:  finishTime.Add(-10 * time.Second),
			FinishTime: finishTime,
			Reason:     model.BuildReasonFlagChangedFiles,
		},
	}
	rs.SetManifestTarget(mt)

	ctx := context.Background()

	// Call OnChange multiple times with the same build
	for i := 0; i < 5; i++ {
		err := collector.OnChange(ctx, rs, store.ChangeSummary{})
		require.NoError(t, err)
	}

	// Should only have one metric recorded
	metrics := mockSt.getMetrics()
	assert.Len(t, metrics, 1)
}

func TestCollector_OnChange_RecordsNewBuilds(t *testing.T) {
	mockSt := newMockStore()
	collector := NewCollector(mockSt)

	rs := newFakeRStore()
	ctx := context.Background()

	m := model.Manifest{Name: "test-service"}
	mt := store.NewManifestTarget(m)
	rs.SetManifestTarget(mt)

	// First build
	firstFinish := time.Now()
	mt.State.BuildHistory = []model.BuildRecord{
		{
			StartTime:  firstFinish.Add(-10 * time.Second),
			FinishTime: firstFinish,
		},
	}
	err := collector.OnChange(ctx, rs, store.ChangeSummary{})
	require.NoError(t, err)

	// Second build (newer)
	secondFinish := firstFinish.Add(30 * time.Second)
	mt.State.BuildHistory = []model.BuildRecord{
		{
			StartTime:  secondFinish.Add(-5 * time.Second),
			FinishTime: secondFinish,
		},
	}
	err = collector.OnChange(ctx, rs, store.ChangeSummary{})
	require.NoError(t, err)

	// Should have two metrics
	metrics := mockSt.getMetrics()
	assert.Len(t, metrics, 2)
}

func TestCollector_OnChange_HandlesFailedBuilds(t *testing.T) {
	mockSt := newMockStore()
	collector := NewCollector(mockSt)

	rs := newFakeRStore()

	// Create a manifest with a failed build
	m := model.Manifest{Name: "failing-service"}
	mt := store.NewManifestTarget(m)
	mt.State.BuildHistory = []model.BuildRecord{
		{
			StartTime:  time.Now().Add(-10 * time.Second),
			FinishTime: time.Now(),
			Error:      assert.AnError,
		},
	}
	rs.SetManifestTarget(mt)

	ctx := context.Background()
	err := collector.OnChange(ctx, rs, store.ChangeSummary{})
	require.NoError(t, err)

	metrics := mockSt.getMetrics()
	require.Len(t, metrics, 1)
	assert.False(t, metrics[0].Success)
	assert.NotEmpty(t, metrics[0].ErrorMessage)
}

func TestCollector_OnChange_DetectsLiveUpdate(t *testing.T) {
	mockSt := newMockStore()
	collector := NewCollector(mockSt)

	rs := newFakeRStore()

	// Create a manifest with a live update build
	m := model.Manifest{Name: "live-update-service"}
	mt := store.NewManifestTarget(m)
	mt.State.BuildHistory = []model.BuildRecord{
		{
			StartTime:  time.Now().Add(-2 * time.Second),
			FinishTime: time.Now(),
			BuildTypes: []model.BuildType{model.BuildTypeLiveUpdate},
		},
	}
	rs.SetManifestTarget(mt)

	ctx := context.Background()
	err := collector.OnChange(ctx, rs, store.ChangeSummary{})
	require.NoError(t, err)

	metrics := mockSt.getMetrics()
	require.Len(t, metrics, 1)
	assert.True(t, metrics[0].LiveUpdate)
}

func TestCollector_Close(t *testing.T) {
	mockSt := newMockStore()
	collector := NewCollector(mockSt)

	err := collector.Close()
	require.NoError(t, err)
}

func TestBuildRecordToMetric(t *testing.T) {
	collector := NewCollector(nil)

	br := model.BuildRecord{
		StartTime:    time.Now().Add(-30 * time.Second),
		FinishTime:   time.Now(),
		Error:        nil,
		WarningCount: 2,
		Reason:       model.BuildReasonFlagChangedFiles,
		BuildTypes:   []model.BuildType{model.BuildTypeImage, model.BuildTypeK8s},
		Edits:        []string{"file1.go", "file2.go", "file3.go"},
	}

	metric := collector.buildRecordToMetric("my-service", br)

	assert.Equal(t, model.ManifestName("my-service"), metric.ManifestName)
	assert.True(t, metric.Success)
	assert.Equal(t, 2, metric.WarningCount)
	assert.Equal(t, 3, metric.FilesChanged)
	assert.False(t, metric.LiveUpdate)
	assert.Equal(t, model.BuildReasonFlagChangedFiles, metric.Reason)
	assert.InDelta(t, 30000, metric.DurationMs, 100)
}
