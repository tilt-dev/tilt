package buildinsights

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/pkg/model"
)

// fakeXDGBase implements xdg.Base for testing.
type fakeXDGBase struct {
	dir string
}

func newFakeXDGBase(t *testing.T) *fakeXDGBase {
	dir := t.TempDir()
	return &fakeXDGBase{dir: dir}
}

func (f *fakeXDGBase) CacheFile(relPath string) (string, error) {
	return filepath.Join(f.dir, "cache", relPath), nil
}

func (f *fakeXDGBase) ConfigFile(relPath string) (string, error) {
	return filepath.Join(f.dir, "config", relPath), nil
}

func (f *fakeXDGBase) DataFile(relPath string) (string, error) {
	path := filepath.Join(f.dir, "data", relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return "", err
	}
	return path, nil
}

func (f *fakeXDGBase) StateFile(relPath string) (string, error) {
	return filepath.Join(f.dir, "state", relPath), nil
}

func (f *fakeXDGBase) RuntimeFile(relPath string) (string, error) {
	return filepath.Join(f.dir, "runtime", relPath), nil
}

func TestFileStore_RecordAndRetrieve(t *testing.T) {
	xdg := newFakeXDGBase(t)
	store, err := NewFileStore(xdg)
	require.NoError(t, err)
	defer store.Close()

	// Record a build
	metric := model.BuildMetric{
		BuildID:      "test-build-1",
		ManifestName: "frontend",
		BuildTypes:   []model.BuildType{model.BuildTypeImage},
		StartTime:    time.Now().Add(-5 * time.Second),
		FinishTime:   time.Now(),
		DurationMs:   5000,
		Success:      true,
		Reason:       model.BuildReasonFlagChangedFiles,
		FilesChanged: 3,
	}

	err = store.RecordBuild(metric)
	require.NoError(t, err)

	// Retrieve recent builds
	builds, err := store.GetRecentBuilds(10)
	require.NoError(t, err)
	assert.Len(t, builds, 1)
	assert.Equal(t, "frontend", string(builds[0].ManifestName))
}

func TestFileStore_GetInsights(t *testing.T) {
	xdg := newFakeXDGBase(t)
	store, err := NewFileStore(xdg)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now()

	// Record multiple builds for different resources
	builds := []model.BuildMetric{
		{
			ManifestName: "frontend",
			StartTime:    now.Add(-10 * time.Minute),
			FinishTime:   now.Add(-9 * time.Minute),
			DurationMs:   60000,
			Success:      true,
			LiveUpdate:   false,
		},
		{
			ManifestName: "frontend",
			StartTime:    now.Add(-5 * time.Minute),
			FinishTime:   now.Add(-4*time.Minute - 30*time.Second),
			DurationMs:   30000,
			Success:      true,
			LiveUpdate:   true,
		},
		{
			ManifestName: "backend",
			StartTime:    now.Add(-3 * time.Minute),
			FinishTime:   now.Add(-2 * time.Minute),
			DurationMs:   60000,
			Success:      false,
		},
		{
			ManifestName: "backend",
			StartTime:    now.Add(-1 * time.Minute),
			FinishTime:   now,
			DurationMs:   45000,
			Success:      true,
		},
	}

	for _, b := range builds {
		err := store.RecordBuild(b)
		require.NoError(t, err)
	}

	// Get insights
	insights, err := store.GetInsights(now.Add(-1 * time.Hour))
	require.NoError(t, err)

	// Verify session stats
	assert.Equal(t, 4, insights.Session.TotalBuilds)
	assert.Equal(t, 3, insights.Session.SuccessfulBuilds)
	assert.Equal(t, 1, insights.Session.FailedBuilds)
	assert.Equal(t, 2, insights.Session.ResourceCount)

	// Verify resource stats
	assert.Len(t, insights.Resources, 2)

	// Find frontend stats
	var frontendStats *model.ResourceStats
	for _, rs := range insights.Resources {
		if rs.ManifestName == "frontend" {
			frontendStats = &rs
			break
		}
	}

	require.NotNil(t, frontendStats)
	assert.Equal(t, 2, frontendStats.TotalBuilds)
	assert.Equal(t, 2, frontendStats.SuccessfulBuilds)
	assert.Equal(t, float64(100), frontendStats.SuccessRate)
	assert.Equal(t, 1, frontendStats.LiveUpdateCount)
}

func TestFileStore_GetResourceStats(t *testing.T) {
	xdg := newFakeXDGBase(t)
	store, err := NewFileStore(xdg)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now()

	// Record builds for one resource
	for i := 0; i < 10; i++ {
		metric := model.BuildMetric{
			ManifestName: "api-server",
			StartTime:    now.Add(time.Duration(-i) * time.Minute),
			FinishTime:   now.Add(time.Duration(-i)*time.Minute + 30*time.Second),
			DurationMs:   30000 + int64(i*1000), // Varying durations
			Success:      i%3 != 0,              // 60% success rate (i=0,3,6,9 are failures)
		}
		err := store.RecordBuild(metric)
		require.NoError(t, err)
	}

	// Get stats for this resource
	stats, err := store.GetResourceStats("api-server")
	require.NoError(t, err)

	assert.Equal(t, model.ManifestName("api-server"), stats.ManifestName)
	assert.Equal(t, 10, stats.TotalBuilds)
	assert.Equal(t, 6, stats.SuccessfulBuilds) // i=1,2,4,5,7,8 are successes
	assert.Equal(t, 4, stats.FailedBuilds)     // i=0,3,6,9 are failures
	assert.InDelta(t, 60.0, stats.SuccessRate, 0.1)

	// Verify duration statistics
	assert.Equal(t, int64(30000), stats.MinDurationMs)
	assert.Equal(t, int64(39000), stats.MaxDurationMs)
}

func TestFileStore_Recommendations(t *testing.T) {
	xdg := newFakeXDGBase(t)
	store, err := NewFileStore(xdg)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now()

	// Create a resource with slow builds that should trigger recommendations
	for i := 0; i < 10; i++ {
		metric := model.BuildMetric{
			ManifestName: "slow-service",
			StartTime:    now.Add(time.Duration(-i) * time.Minute),
			FinishTime:   now.Add(time.Duration(-i)*time.Minute + 45*time.Second),
			DurationMs:   45000, // 45 seconds - slow enough to trigger recommendation
			Success:      true,
			LiveUpdate:   false, // All full rebuilds
		}
		err := store.RecordBuild(metric)
		require.NoError(t, err)
	}

	insights, err := store.GetInsights(now.Add(-1 * time.Hour))
	require.NoError(t, err)

	// Should have a recommendation for slow builds
	hasPerformanceRec := false
	for _, rec := range insights.Recommendations {
		if rec.Type == model.RecommendationTypePerformance {
			hasPerformanceRec = true
			assert.Contains(t, rec.Description, "slow-service")
			break
		}
	}
	assert.True(t, hasPerformanceRec, "Expected a performance recommendation for slow builds")
}

func TestFileStore_HighFailureRateRecommendation(t *testing.T) {
	xdg := newFakeXDGBase(t)
	store, err := NewFileStore(xdg)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now()

	// Create a resource with high failure rate
	for i := 0; i < 10; i++ {
		metric := model.BuildMetric{
			ManifestName: "flaky-service",
			StartTime:    now.Add(time.Duration(-i) * time.Minute),
			FinishTime:   now.Add(time.Duration(-i)*time.Minute + 10*time.Second),
			DurationMs:   10000,
			Success:      i < 3, // Only 30% success rate
		}
		err := store.RecordBuild(metric)
		require.NoError(t, err)
	}

	insights, err := store.GetInsights(now.Add(-1 * time.Hour))
	require.NoError(t, err)

	// Should have a reliability recommendation
	hasReliabilityRec := false
	for _, rec := range insights.Recommendations {
		if rec.Type == model.RecommendationTypeReliability {
			hasReliabilityRec = true
			assert.Contains(t, rec.Description, "flaky-service")
			break
		}
	}
	assert.True(t, hasReliabilityRec, "Expected a reliability recommendation for high failure rate")
}

func TestFileStore_Persistence(t *testing.T) {
	xdg := newFakeXDGBase(t)

	// Create store and add data
	store1, err := NewFileStore(xdg)
	require.NoError(t, err)

	metric := model.BuildMetric{
		ManifestName: "persistent-service",
		StartTime:    time.Now().Add(-5 * time.Minute),
		FinishTime:   time.Now(),
		DurationMs:   300000,
		Success:      true,
	}
	err = store1.RecordBuild(metric)
	require.NoError(t, err)

	// Close and reopen
	err = store1.Close()
	require.NoError(t, err)

	store2, err := NewFileStore(xdg)
	require.NoError(t, err)
	defer store2.Close()

	// Data should still be there
	builds, err := store2.GetRecentBuilds(10)
	require.NoError(t, err)
	assert.Len(t, builds, 1)
	assert.Equal(t, "persistent-service", string(builds[0].ManifestName))
}

func TestFileStore_ClearOlderThan(t *testing.T) {
	xdg := newFakeXDGBase(t)
	store, err := NewFileStore(xdg)
	require.NoError(t, err)
	defer store.Close()

	now := time.Now()

	// Add old and new builds
	oldBuild := model.BuildMetric{
		ManifestName: "old-build",
		StartTime:    now.Add(-48 * time.Hour),
		FinishTime:   now.Add(-48*time.Hour + 10*time.Second),
		DurationMs:   10000,
		Success:      true,
	}
	newBuild := model.BuildMetric{
		ManifestName: "new-build",
		StartTime:    now.Add(-1 * time.Hour),
		FinishTime:   now.Add(-1*time.Hour + 10*time.Second),
		DurationMs:   10000,
		Success:      true,
	}

	require.NoError(t, store.RecordBuild(oldBuild))
	require.NoError(t, store.RecordBuild(newBuild))

	// Clear old builds
	err = store.ClearOlderThan(now.Add(-24 * time.Hour))
	require.NoError(t, err)

	// Only new build should remain
	builds, err := store.GetRecentBuilds(10)
	require.NoError(t, err)
	assert.Len(t, builds, 1)
	assert.Equal(t, "new-build", string(builds[0].ManifestName))
}

func TestPercentile(t *testing.T) {
	testCases := []struct {
		name     string
		values   []int64
		n        int
		expected int64
	}{
		{
			name:     "empty slice",
			values:   []int64{},
			n:        50,
			expected: 0,
		},
		{
			name:     "single value",
			values:   []int64{100},
			n:        50,
			expected: 100,
		},
		{
			name:     "p50 of sorted values",
			values:   []int64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
			n:        50,
			expected: 60, // idx = (50 * 10) / 100 = 5 -> values[5] = 60
		},
		{
			name:     "p95 of sorted values",
			values:   []int64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
			n:        95,
			expected: 100,
		},
		{
			name:     "p99 of sorted values",
			values:   []int64{10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
			n:        99,
			expected: 100,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := percentile(tc.values, tc.n)
			assert.Equal(t, tc.expected, result)
		})
	}
}
