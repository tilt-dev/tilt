package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/tilt-dev/tilt/pkg/model"
)

// InsightsHandler handles HTTP requests for build insights.
type InsightsHandler struct {
	store model.BuildInsightsStore
}

// NewInsightsHandler creates a new insights HTTP handler.
func NewInsightsHandler(store model.BuildInsightsStore) *InsightsHandler {
	return &InsightsHandler{store: store}
}

// insightsQueryParams represents query parameters for insights requests.
type insightsQueryParams struct {
	Since    time.Time
	Resource string
	Limit    int
}

// parseQueryParams extracts query parameters from the request.
func parseQueryParams(r *http.Request) insightsQueryParams {
	params := insightsQueryParams{
		Since: time.Now().Add(-24 * time.Hour), // Default to last 24 hours
		Limit: 100,
	}

	if sinceStr := r.URL.Query().Get("since"); sinceStr != "" {
		// Try parsing as duration (e.g., "24h", "7d")
		if d, err := time.ParseDuration(sinceStr); err == nil {
			params.Since = time.Now().Add(-d)
		} else if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			// Try parsing as RFC3339 timestamp
			params.Since = t
		}
	}

	// Handle special "days" format
	if daysStr := r.URL.Query().Get("days"); daysStr != "" {
		if days, err := strconv.Atoi(daysStr); err == nil && days > 0 {
			params.Since = time.Now().Add(-time.Duration(days) * 24 * time.Hour)
		}
	}

	if resource := r.URL.Query().Get("resource"); resource != "" {
		params.Resource = resource
	}

	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			params.Limit = limit
		}
	}

	return params
}

// HandleInsights returns comprehensive build insights.
// GET /api/insights
// Query params:
//   - since: RFC3339 timestamp or duration (e.g., "24h", "168h")
//   - days: number of days to look back (alternative to since)
func (h *InsightsHandler) HandleInsights(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "must be GET request", http.StatusMethodNotAllowed)
		return
	}

	if h.store == nil {
		http.Error(w, "insights not available", http.StatusServiceUnavailable)
		return
	}

	params := parseQueryParams(r)

	insights, err := h.store.GetInsights(params.Since)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get insights: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(insights); err != nil {
		http.Error(w, fmt.Sprintf("failed to encode response: %v", err), http.StatusInternalServerError)
	}
}

// HandleResourceStats returns statistics for a specific resource.
// GET /api/insights/resource/{name}
func (h *InsightsHandler) HandleResourceStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "must be GET request", http.StatusMethodNotAllowed)
		return
	}

	if h.store == nil {
		http.Error(w, "insights not available", http.StatusServiceUnavailable)
		return
	}

	// Extract resource name from URL path
	// Expected path: /api/insights/resource/{name}
	resourceName := r.URL.Query().Get("name")
	if resourceName == "" {
		http.Error(w, "resource name is required (use ?name=<resource>)", http.StatusBadRequest)
		return
	}

	stats, err := h.store.GetResourceStats(model.ManifestName(resourceName))
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get resource stats: %v", err), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(stats); err != nil {
		http.Error(w, fmt.Sprintf("failed to encode response: %v", err), http.StatusInternalServerError)
	}
}

// HandleRecentBuilds returns the most recent builds.
// GET /api/insights/builds
// Query params:
//   - limit: number of builds to return (default: 100)
//   - resource: filter by resource name (optional)
func (h *InsightsHandler) HandleRecentBuilds(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "must be GET request", http.StatusMethodNotAllowed)
		return
	}

	if h.store == nil {
		http.Error(w, "insights not available", http.StatusServiceUnavailable)
		return
	}

	params := parseQueryParams(r)

	var builds []model.BuildMetric
	var err error

	if params.Resource != "" {
		builds, err = h.store.GetBuildHistory(model.ManifestName(params.Resource), params.Since)
	} else {
		builds, err = h.store.GetRecentBuilds(params.Limit)
	}

	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get builds: %v", err), http.StatusInternalServerError)
		return
	}

	// Apply limit
	if len(builds) > params.Limit {
		builds = builds[:params.Limit]
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(builds); err != nil {
		http.Error(w, fmt.Sprintf("failed to encode response: %v", err), http.StatusInternalServerError)
	}
}

// HandleSummary returns a brief summary suitable for CLI output.
// GET /api/insights/summary
func (h *InsightsHandler) HandleSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "must be GET request", http.StatusMethodNotAllowed)
		return
	}

	if h.store == nil {
		http.Error(w, "insights not available", http.StatusServiceUnavailable)
		return
	}

	params := parseQueryParams(r)

	insights, err := h.store.GetInsights(params.Since)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to get insights: %v", err), http.StatusInternalServerError)
		return
	}

	// Create a condensed summary
	summary := map[string]interface{}{
		"session": map[string]interface{}{
			"total_builds":      insights.Session.TotalBuilds,
			"success_rate":      calculateSuccessRate(insights.Session.SuccessfulBuilds, insights.Session.TotalBuilds),
			"avg_duration_ms":   insights.Session.AverageDurationMs,
			"total_duration_ms": insights.Session.TotalDurationMs,
			"live_updates":      insights.Session.LiveUpdateCount,
			"full_rebuilds":     insights.Session.FullRebuildCount,
			"resource_count":    insights.Session.ResourceCount,
		},
		"recommendations_count": len(insights.Recommendations),
		"failed_resources":      len(insights.MostFailedResources),
	}

	// Add top recommendations
	if len(insights.Recommendations) > 0 {
		topRecs := insights.Recommendations
		if len(topRecs) > 3 {
			topRecs = topRecs[:3]
		}
		summary["top_recommendations"] = topRecs
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(summary); err != nil {
		http.Error(w, fmt.Sprintf("failed to encode response: %v", err), http.StatusInternalServerError)
	}
}

func calculateSuccessRate(successful, total int) float64 {
	if total == 0 {
		return 0
	}
	return float64(successful) / float64(total) * 100
}
