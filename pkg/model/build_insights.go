package model

import (
	"encoding/json"
	"time"
)

// BuildInsightsVersion is the current version of the insights data format.
// Increment this when making breaking changes to the data structure.
const BuildInsightsVersion = 1

// BuildMetric represents a single build execution with all relevant metrics.
type BuildMetric struct {
	// Unique identifier for this build
	BuildID string `json:"build_id"`

	// ManifestName is the name of the manifest that was built
	ManifestName ManifestName `json:"manifest_name"`

	// BuildTypes indicates what kind of build was performed
	BuildTypes []BuildType `json:"build_types"`

	// StartTime is when the build started
	StartTime time.Time `json:"start_time"`

	// FinishTime is when the build completed
	FinishTime time.Time `json:"finish_time"`

	// Duration in milliseconds for easier aggregation
	DurationMs int64 `json:"duration_ms"`

	// Success indicates whether the build succeeded
	Success bool `json:"success"`

	// ErrorMessage contains the error if the build failed
	ErrorMessage string `json:"error_message,omitempty"`

	// WarningCount is the number of warnings during the build
	WarningCount int `json:"warning_count"`

	// Reason indicates why this build was triggered
	Reason BuildReason `json:"reason"`

	// CacheHit indicates if the build used cached layers (for image builds)
	CacheHit bool `json:"cache_hit"`

	// ImageSize in bytes (for image builds)
	ImageSize int64 `json:"image_size,omitempty"`

	// LiveUpdate indicates if this was a live update instead of full rebuild
	LiveUpdate bool `json:"live_update"`

	// FilesChanged is the count of files that triggered this build
	FilesChanged int `json:"files_changed"`
}

// Duration returns the build duration as a time.Duration.
func (bm BuildMetric) Duration() time.Duration {
	return time.Duration(bm.DurationMs) * time.Millisecond
}

// ResourceStats aggregates statistics for a single resource/manifest.
type ResourceStats struct {
	// ManifestName is the name of the resource
	ManifestName ManifestName `json:"manifest_name"`

	// TotalBuilds is the total number of builds
	TotalBuilds int `json:"total_builds"`

	// SuccessfulBuilds is the count of successful builds
	SuccessfulBuilds int `json:"successful_builds"`

	// FailedBuilds is the count of failed builds
	FailedBuilds int `json:"failed_builds"`

	// SuccessRate as a percentage (0-100)
	SuccessRate float64 `json:"success_rate"`

	// TotalDurationMs is the sum of all build durations
	TotalDurationMs int64 `json:"total_duration_ms"`

	// AverageDurationMs is the mean build duration
	AverageDurationMs int64 `json:"average_duration_ms"`

	// MinDurationMs is the fastest build time
	MinDurationMs int64 `json:"min_duration_ms"`

	// MaxDurationMs is the slowest build time
	MaxDurationMs int64 `json:"max_duration_ms"`

	// P50DurationMs is the median build time
	P50DurationMs int64 `json:"p50_duration_ms"`

	// P95DurationMs is the 95th percentile build time
	P95DurationMs int64 `json:"p95_duration_ms"`

	// P99DurationMs is the 99th percentile build time
	P99DurationMs int64 `json:"p99_duration_ms"`

	// LiveUpdateCount is the number of live updates
	LiveUpdateCount int `json:"live_update_count"`

	// FullRebuildCount is the number of full rebuilds
	FullRebuildCount int `json:"full_rebuild_count"`

	// CacheHitRate as a percentage (0-100)
	CacheHitRate float64 `json:"cache_hit_rate"`

	// TotalWarnings across all builds
	TotalWarnings int `json:"total_warnings"`

	// LastBuildTime is the timestamp of the most recent build
	LastBuildTime time.Time `json:"last_build_time"`

	// LastBuildSuccess indicates if the last build was successful
	LastBuildSuccess bool `json:"last_build_success"`

	// AverageFilesChanged is the mean number of files triggering builds
	AverageFilesChanged float64 `json:"average_files_changed"`
}

// SessionStats aggregates statistics for the current Tilt session.
type SessionStats struct {
	// SessionID uniquely identifies this Tilt session
	SessionID string `json:"session_id"`

	// StartTime is when this Tilt session started
	StartTime time.Time `json:"start_time"`

	// TotalBuilds is the total number of builds in this session
	TotalBuilds int `json:"total_builds"`

	// SuccessfulBuilds count
	SuccessfulBuilds int `json:"successful_builds"`

	// FailedBuilds count
	FailedBuilds int `json:"failed_builds"`

	// TotalDurationMs is the sum of all build times
	TotalDurationMs int64 `json:"total_duration_ms"`

	// AverageDurationMs is the mean build time
	AverageDurationMs int64 `json:"average_duration_ms"`

	// LiveUpdateCount is the number of live updates
	LiveUpdateCount int `json:"live_update_count"`

	// FullRebuildCount is the number of full rebuilds
	FullRebuildCount int `json:"full_rebuild_count"`

	// ResourceCount is the number of unique resources that have been built
	ResourceCount int `json:"resource_count"`

	// CurrentStreak is the current consecutive success/failure streak
	CurrentStreak int `json:"current_streak"`

	// StreakType is "success" or "failure"
	StreakType string `json:"streak_type"`
}

// BuildInsights contains comprehensive build analytics data.
type BuildInsights struct {
	// Version is the data format version
	Version int `json:"version"`

	// GeneratedAt is when this insights report was generated
	GeneratedAt time.Time `json:"generated_at"`

	// Session contains current session statistics
	Session SessionStats `json:"session"`

	// Resources contains per-resource statistics
	Resources []ResourceStats `json:"resources"`

	// RecentBuilds contains the most recent build metrics
	RecentBuilds []BuildMetric `json:"recent_builds"`

	// SlowestBuilds contains the slowest builds for analysis
	SlowestBuilds []BuildMetric `json:"slowest_builds"`

	// MostFailedResources is ordered by failure count
	MostFailedResources []ResourceStats `json:"most_failed_resources"`

	// Recommendations contains actionable insights
	Recommendations []BuildRecommendation `json:"recommendations"`
}

// RecommendationType categorizes the type of recommendation.
type RecommendationType string

const (
	RecommendationTypePerformance RecommendationType = "performance"
	RecommendationTypeReliability RecommendationType = "reliability"
	RecommendationTypeCaching     RecommendationType = "caching"
	RecommendationTypeGeneral     RecommendationType = "general"
)

// RecommendationPriority indicates the urgency of a recommendation.
type RecommendationPriority string

const (
	RecommendationPriorityHigh   RecommendationPriority = "high"
	RecommendationPriorityMedium RecommendationPriority = "medium"
	RecommendationPriorityLow    RecommendationPriority = "low"
)

// BuildRecommendation represents an actionable insight for improving builds.
type BuildRecommendation struct {
	// Type categorizes this recommendation
	Type RecommendationType `json:"type"`

	// Priority indicates urgency
	Priority RecommendationPriority `json:"priority"`

	// Title is a short description
	Title string `json:"title"`

	// Description provides detailed explanation
	Description string `json:"description"`

	// AffectedResources lists resources this applies to
	AffectedResources []ManifestName `json:"affected_resources,omitempty"`

	// PotentialSavingsMs is the estimated time savings if implemented
	PotentialSavingsMs int64 `json:"potential_savings_ms,omitempty"`
}

// BuildInsightsStore defines the interface for persisting build insights.
type BuildInsightsStore interface {
	// RecordBuild stores a new build metric
	RecordBuild(metric BuildMetric) error

	// GetInsights returns computed insights for the specified time range
	GetInsights(since time.Time) (*BuildInsights, error)

	// GetResourceStats returns statistics for a specific resource
	GetResourceStats(name ManifestName) (*ResourceStats, error)

	// GetRecentBuilds returns the most recent n builds
	GetRecentBuilds(n int) ([]BuildMetric, error)

	// GetBuildHistory returns all builds for a resource within a time range
	GetBuildHistory(name ManifestName, since time.Time) ([]BuildMetric, error)

	// ClearOlderThan removes metrics older than the specified time
	ClearOlderThan(t time.Time) error

	// Close releases any resources held by the store
	Close() error
}

// MarshalJSON implements custom JSON marshaling for BuildInsights.
func (bi BuildInsights) MarshalJSON() ([]byte, error) {
	type Alias BuildInsights
	return json.Marshal(&struct {
		Alias
	}{
		Alias: (Alias)(bi),
	})
}
