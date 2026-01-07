package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/pkg/model"
)

// mockInsightsStore implements model.BuildInsightsStore for testing.
type mockInsightsStore struct {
	insights *model.BuildInsights
	stats    *model.ResourceStats
	builds   []model.BuildMetric
	err      error
}

func (m *mockInsightsStore) RecordBuild(metric model.BuildMetric) error {
	return m.err
}

func (m *mockInsightsStore) GetInsights(since time.Time) (*model.BuildInsights, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.insights, nil
}

func (m *mockInsightsStore) GetResourceStats(name model.ManifestName) (*model.ResourceStats, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.stats, nil
}

func (m *mockInsightsStore) GetRecentBuilds(n int) ([]model.BuildMetric, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.builds, nil
}

func (m *mockInsightsStore) GetBuildHistory(name model.ManifestName, since time.Time) ([]model.BuildMetric, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.builds, nil
}

func (m *mockInsightsStore) ClearOlderThan(t time.Time) error {
	return m.err
}

func (m *mockInsightsStore) Close() error {
	return m.err
}

func TestInsightsHandler_HandleInsights(t *testing.T) {
	store := &mockInsightsStore{
		insights: &model.BuildInsights{
			Version:     1,
			GeneratedAt: time.Now(),
			Session: model.SessionStats{
				TotalBuilds:      10,
				SuccessfulBuilds: 9,
				FailedBuilds:     1,
			},
			Resources: []model.ResourceStats{
				{
					ManifestName: "frontend",
					TotalBuilds:  5,
					SuccessRate:  100,
				},
			},
		},
	}

	handler := NewInsightsHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/insights", nil)
	w := httptest.NewRecorder()

	handler.HandleInsights(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))

	var insights model.BuildInsights
	err := json.NewDecoder(resp.Body).Decode(&insights)
	require.NoError(t, err)

	assert.Equal(t, 10, insights.Session.TotalBuilds)
	assert.Len(t, insights.Resources, 1)
	assert.Equal(t, model.ManifestName("frontend"), insights.Resources[0].ManifestName)
}

func TestInsightsHandler_HandleInsights_MethodNotAllowed(t *testing.T) {
	handler := NewInsightsHandler(&mockInsightsStore{})

	req := httptest.NewRequest(http.MethodPost, "/api/insights", nil)
	w := httptest.NewRecorder()

	handler.HandleInsights(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode)
}

func TestInsightsHandler_HandleInsights_NilStore(t *testing.T) {
	handler := NewInsightsHandler(nil)

	req := httptest.NewRequest(http.MethodGet, "/api/insights", nil)
	w := httptest.NewRecorder()

	handler.HandleInsights(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
}

func TestInsightsHandler_HandleResourceStats(t *testing.T) {
	store := &mockInsightsStore{
		stats: &model.ResourceStats{
			ManifestName:      "backend",
			TotalBuilds:       20,
			SuccessfulBuilds:  18,
			AverageDurationMs: 5000,
		},
	}

	handler := NewInsightsHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/insights/resource?name=backend", nil)
	w := httptest.NewRecorder()

	handler.HandleResourceStats(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var stats model.ResourceStats
	err := json.NewDecoder(resp.Body).Decode(&stats)
	require.NoError(t, err)

	assert.Equal(t, model.ManifestName("backend"), stats.ManifestName)
	assert.Equal(t, 20, stats.TotalBuilds)
}

func TestInsightsHandler_HandleResourceStats_MissingName(t *testing.T) {
	handler := NewInsightsHandler(&mockInsightsStore{})

	req := httptest.NewRequest(http.MethodGet, "/api/insights/resource", nil)
	w := httptest.NewRecorder()

	handler.HandleResourceStats(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestInsightsHandler_HandleRecentBuilds(t *testing.T) {
	store := &mockInsightsStore{
		builds: []model.BuildMetric{
			{
				ManifestName: "service-1",
				DurationMs:   5000,
				Success:      true,
			},
			{
				ManifestName: "service-2",
				DurationMs:   3000,
				Success:      false,
			},
		},
	}

	handler := NewInsightsHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/insights/builds?limit=10", nil)
	w := httptest.NewRecorder()

	handler.HandleRecentBuilds(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var builds []model.BuildMetric
	err := json.NewDecoder(resp.Body).Decode(&builds)
	require.NoError(t, err)

	assert.Len(t, builds, 2)
}

func TestInsightsHandler_HandleSummary(t *testing.T) {
	store := &mockInsightsStore{
		insights: &model.BuildInsights{
			Session: model.SessionStats{
				TotalBuilds:       50,
				SuccessfulBuilds:  45,
				AverageDurationMs: 10000,
			},
			Recommendations: []model.BuildRecommendation{
				{
					Type:     model.RecommendationTypePerformance,
					Priority: model.RecommendationPriorityHigh,
					Title:    "Test recommendation",
				},
			},
		},
	}

	handler := NewInsightsHandler(store)

	req := httptest.NewRequest(http.MethodGet, "/api/insights/summary", nil)
	w := httptest.NewRecorder()

	handler.HandleSummary(w, req)

	resp := w.Result()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	var summary map[string]interface{}
	err := json.NewDecoder(resp.Body).Decode(&summary)
	require.NoError(t, err)

	session := summary["session"].(map[string]interface{})
	assert.Equal(t, float64(50), session["total_builds"])
	assert.Equal(t, float64(1), summary["recommendations_count"])
}

func TestParseQueryParams(t *testing.T) {
	testCases := []struct {
		name          string
		url           string
		expectedSince time.Duration // approximate time ago
		expectedLimit int
	}{
		{
			name:          "default values",
			url:           "/api/insights",
			expectedSince: 24 * time.Hour,
			expectedLimit: 100,
		},
		{
			name:          "custom since duration",
			url:           "/api/insights?since=48h",
			expectedSince: 48 * time.Hour,
			expectedLimit: 100,
		},
		{
			name:          "days parameter",
			url:           "/api/insights?days=7",
			expectedSince: 7 * 24 * time.Hour,
			expectedLimit: 100,
		},
		{
			name:          "custom limit",
			url:           "/api/insights?limit=50",
			expectedSince: 24 * time.Hour,
			expectedLimit: 50,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.url, nil)
			params := parseQueryParams(req)

			// Check that since is approximately correct (within 1 second tolerance)
			expectedSince := time.Now().Add(-tc.expectedSince)
			assert.WithinDuration(t, expectedSince, params.Since, time.Second)

			assert.Equal(t, tc.expectedLimit, params.Limit)
		})
	}
}
