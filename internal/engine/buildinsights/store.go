package buildinsights

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/tilt-dev/tilt/internal/xdg"
	"github.com/tilt-dev/tilt/pkg/model"
)

const (
	// insightsDir is the directory within XDG data directory for insights storage
	insightsDir = "insights"

	// metricsFileName is the name of the metrics database file
	metricsFileName = "build_metrics.json"

	// maxMetricsAge is the default maximum age for stored metrics (30 days)
	maxMetricsAge = 30 * 24 * time.Hour

	// maxRecentBuilds is the maximum number of recent builds to return
	maxRecentBuilds = 100

	// maxSlowestBuilds is the number of slowest builds to track
	maxSlowestBuilds = 10
)

// FileStore implements BuildInsightsStore using local file storage.
// It persists metrics to a JSON file in the XDG data directory.
type FileStore struct {
	mu        sync.RWMutex
	xdgBase   xdg.Base
	sessionID string
	startTime time.Time

	// In-memory cache of metrics
	metrics   []model.BuildMetric
	filePath  string
	dirty     bool
	lastFlush time.Time
}

// NewFileStore creates a new file-based insights store.
func NewFileStore(xdgBase xdg.Base) (*FileStore, error) {
	filePath, err := xdgBase.DataFile(filepath.Join(insightsDir, metricsFileName))
	if err != nil {
		return nil, fmt.Errorf("failed to get insights data path: %w", err)
	}

	store := &FileStore{
		xdgBase:   xdgBase,
		sessionID: uuid.New().String(),
		startTime: time.Now(),
		metrics:   make([]model.BuildMetric, 0),
		filePath:  filePath,
		lastFlush: time.Now(),
	}

	// Load existing metrics
	if err := store.load(); err != nil {
		// If file doesn't exist, that's OK - we'll create it on first write
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("failed to load existing metrics: %w", err)
		}
	}

	return store, nil
}

// metricsFile represents the on-disk format of the metrics database.
type metricsFile struct {
	Version int                 `json:"version"`
	Metrics []model.BuildMetric `json:"metrics"`
}

// load reads metrics from the file system.
func (fs *FileStore) load() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	data, err := os.ReadFile(fs.filePath)
	if err != nil {
		return err
	}

	var mf metricsFile
	if err := json.Unmarshal(data, &mf); err != nil {
		return fmt.Errorf("failed to unmarshal metrics file: %w", err)
	}

	// Handle version migrations if needed
	if mf.Version != model.BuildInsightsVersion {
		// For now, just accept older versions as-is
		// In future, add migration logic here
	}

	fs.metrics = mf.Metrics
	return nil
}

// flush writes metrics to the file system.
func (fs *FileStore) flush() error {
	if !fs.dirty {
		return nil
	}

	mf := metricsFile{
		Version: model.BuildInsightsVersion,
		Metrics: fs.metrics,
	}

	data, err := json.MarshalIndent(mf, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metrics: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(fs.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create insights directory: %w", err)
	}

	// Write atomically using temp file
	tmpPath := fs.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write metrics file: %w", err)
	}

	if err := os.Rename(tmpPath, fs.filePath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("failed to rename temp metrics file: %w", err)
	}

	fs.dirty = false
	fs.lastFlush = time.Now()
	return nil
}

// RecordBuild stores a new build metric.
func (fs *FileStore) RecordBuild(metric model.BuildMetric) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	// Generate ID if not set
	if metric.BuildID == "" {
		metric.BuildID = uuid.New().String()
	}

	fs.metrics = append(fs.metrics, metric)
	fs.dirty = true

	// Auto-flush periodically to avoid losing data
	if time.Since(fs.lastFlush) > time.Minute {
		return fs.flush()
	}

	return nil
}

// GetInsights returns computed insights for the specified time range.
func (fs *FileStore) GetInsights(since time.Time) (*model.BuildInsights, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	// Filter metrics by time range
	var filtered []model.BuildMetric
	for _, m := range fs.metrics {
		if m.StartTime.After(since) || m.StartTime.Equal(since) {
			filtered = append(filtered, m)
		}
	}

	// Compute per-resource stats
	resourceMetrics := make(map[model.ManifestName][]model.BuildMetric)
	for _, m := range filtered {
		resourceMetrics[m.ManifestName] = append(resourceMetrics[m.ManifestName], m)
	}

	var resourceStats []model.ResourceStats
	for name, metrics := range resourceMetrics {
		stats := fs.computeResourceStats(name, metrics)
		resourceStats = append(resourceStats, stats)
	}

	// Sort by total builds descending
	sort.Slice(resourceStats, func(i, j int) bool {
		return resourceStats[i].TotalBuilds > resourceStats[j].TotalBuilds
	})

	// Get recent builds
	recentBuilds := fs.getRecentBuildsLocked(maxRecentBuilds, filtered)

	// Get slowest builds
	slowestBuilds := fs.getSlowestBuildsLocked(maxSlowestBuilds, filtered)

	// Get most failed resources
	mostFailed := fs.getMostFailedResources(resourceStats, 5)

	// Compute session stats
	sessionStats := fs.computeSessionStats(filtered)

	// Generate recommendations
	recommendations := fs.generateRecommendations(resourceStats, filtered)

	return &model.BuildInsights{
		Version:             model.BuildInsightsVersion,
		GeneratedAt:         time.Now(),
		Session:             sessionStats,
		Resources:           resourceStats,
		RecentBuilds:        recentBuilds,
		SlowestBuilds:       slowestBuilds,
		MostFailedResources: mostFailed,
		Recommendations:     recommendations,
	}, nil
}

// computeResourceStats calculates statistics for a single resource.
func (fs *FileStore) computeResourceStats(name model.ManifestName, metrics []model.BuildMetric) model.ResourceStats {
	if len(metrics) == 0 {
		return model.ResourceStats{ManifestName: name}
	}

	var (
		totalDuration   int64
		successCount    int
		failedCount     int
		liveUpdateCount int
		cacheHitCount   int
		totalWarnings   int
		totalFiles      int
		durations       []int64
		lastBuild       model.BuildMetric
	)

	for _, m := range metrics {
		durations = append(durations, m.DurationMs)
		totalDuration += m.DurationMs
		totalWarnings += m.WarningCount
		totalFiles += m.FilesChanged

		if m.Success {
			successCount++
		} else {
			failedCount++
		}

		if m.LiveUpdate {
			liveUpdateCount++
		}

		if m.CacheHit {
			cacheHitCount++
		}

		if m.StartTime.After(lastBuild.StartTime) {
			lastBuild = m
		}
	}

	sort.Slice(durations, func(i, j int) bool {
		return durations[i] < durations[j]
	})

	total := len(metrics)
	avgDuration := totalDuration / int64(total)
	avgFiles := float64(totalFiles) / float64(total)

	successRate := float64(0)
	if total > 0 {
		successRate = float64(successCount) / float64(total) * 100
	}

	cacheHitRate := float64(0)
	if total > 0 {
		cacheHitRate = float64(cacheHitCount) / float64(total) * 100
	}

	return model.ResourceStats{
		ManifestName:        name,
		TotalBuilds:         total,
		SuccessfulBuilds:    successCount,
		FailedBuilds:        failedCount,
		SuccessRate:         successRate,
		TotalDurationMs:     totalDuration,
		AverageDurationMs:   avgDuration,
		MinDurationMs:       durations[0],
		MaxDurationMs:       durations[len(durations)-1],
		P50DurationMs:       percentile(durations, 50),
		P95DurationMs:       percentile(durations, 95),
		P99DurationMs:       percentile(durations, 99),
		LiveUpdateCount:     liveUpdateCount,
		FullRebuildCount:    total - liveUpdateCount,
		CacheHitRate:        cacheHitRate,
		TotalWarnings:       totalWarnings,
		LastBuildTime:       lastBuild.StartTime,
		LastBuildSuccess:    lastBuild.Success,
		AverageFilesChanged: avgFiles,
	}
}

// percentile calculates the nth percentile from a sorted slice.
func percentile(sorted []int64, n int) int64 {
	if len(sorted) == 0 {
		return 0
	}
	idx := (n * len(sorted)) / 100
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

// computeSessionStats calculates statistics for the current session.
func (fs *FileStore) computeSessionStats(metrics []model.BuildMetric) model.SessionStats {
	var (
		totalDuration    int64
		successCount     int
		failedCount      int
		liveUpdateCount  int
		resourceSet      = make(map[model.ManifestName]bool)
		currentStreak    int
		streakType       string
		lastSuccess      *bool
	)

	// Sort by time to calculate streaks
	sorted := make([]model.BuildMetric, len(metrics))
	copy(sorted, metrics)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].StartTime.Before(sorted[j].StartTime)
	})

	for _, m := range sorted {
		totalDuration += m.DurationMs
		resourceSet[m.ManifestName] = true

		if m.Success {
			successCount++
		} else {
			failedCount++
		}

		if m.LiveUpdate {
			liveUpdateCount++
		}

		// Calculate streak
		if lastSuccess == nil {
			currentStreak = 1
			if m.Success {
				streakType = "success"
			} else {
				streakType = "failure"
			}
		} else if *lastSuccess == m.Success {
			currentStreak++
		} else {
			currentStreak = 1
			if m.Success {
				streakType = "success"
			} else {
				streakType = "failure"
			}
		}
		s := m.Success
		lastSuccess = &s
	}

	total := len(metrics)
	avgDuration := int64(0)
	if total > 0 {
		avgDuration = totalDuration / int64(total)
	}

	return model.SessionStats{
		SessionID:         fs.sessionID,
		StartTime:         fs.startTime,
		TotalBuilds:       total,
		SuccessfulBuilds:  successCount,
		FailedBuilds:      failedCount,
		TotalDurationMs:   totalDuration,
		AverageDurationMs: avgDuration,
		LiveUpdateCount:   liveUpdateCount,
		FullRebuildCount:  total - liveUpdateCount,
		ResourceCount:     len(resourceSet),
		CurrentStreak:     currentStreak,
		StreakType:        streakType,
	}
}

// getRecentBuildsLocked returns the most recent n builds.
func (fs *FileStore) getRecentBuildsLocked(n int, metrics []model.BuildMetric) []model.BuildMetric {
	sorted := make([]model.BuildMetric, len(metrics))
	copy(sorted, metrics)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].StartTime.After(sorted[j].StartTime)
	})

	if len(sorted) > n {
		sorted = sorted[:n]
	}
	return sorted
}

// getSlowestBuildsLocked returns the n slowest builds.
func (fs *FileStore) getSlowestBuildsLocked(n int, metrics []model.BuildMetric) []model.BuildMetric {
	sorted := make([]model.BuildMetric, len(metrics))
	copy(sorted, metrics)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].DurationMs > sorted[j].DurationMs
	})

	if len(sorted) > n {
		sorted = sorted[:n]
	}
	return sorted
}

// getMostFailedResources returns resources with most failures.
func (fs *FileStore) getMostFailedResources(stats []model.ResourceStats, n int) []model.ResourceStats {
	sorted := make([]model.ResourceStats, len(stats))
	copy(sorted, stats)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].FailedBuilds > sorted[j].FailedBuilds
	})

	// Filter to only resources with failures
	var withFailures []model.ResourceStats
	for _, s := range sorted {
		if s.FailedBuilds > 0 {
			withFailures = append(withFailures, s)
		}
	}

	if len(withFailures) > n {
		withFailures = withFailures[:n]
	}
	return withFailures
}

// generateRecommendations creates actionable insights from the data.
func (fs *FileStore) generateRecommendations(stats []model.ResourceStats, metrics []model.BuildMetric) []model.BuildRecommendation {
	var recommendations []model.BuildRecommendation

	for _, s := range stats {
		// Slow builds recommendation
		if s.AverageDurationMs > 30000 && s.LiveUpdateCount < s.FullRebuildCount {
			recommendations = append(recommendations, model.BuildRecommendation{
				Type:               model.RecommendationTypePerformance,
				Priority:           model.RecommendationPriorityHigh,
				Title:              "Consider enabling Live Update",
				Description:        fmt.Sprintf("Resource '%s' has slow builds (avg %.1fs) with mostly full rebuilds. Live Update could significantly reduce iteration time.", s.ManifestName, float64(s.AverageDurationMs)/1000),
				AffectedResources:  []model.ManifestName{s.ManifestName},
				PotentialSavingsMs: s.AverageDurationMs / 2,
			})
		}

		// High failure rate recommendation
		if s.TotalBuilds >= 5 && s.SuccessRate < 70 {
			recommendations = append(recommendations, model.BuildRecommendation{
				Type:              model.RecommendationTypeReliability,
				Priority:          model.RecommendationPriorityHigh,
				Title:             "High build failure rate detected",
				Description:       fmt.Sprintf("Resource '%s' has a %.0f%% success rate. Review build configuration and error logs.", s.ManifestName, s.SuccessRate),
				AffectedResources: []model.ManifestName{s.ManifestName},
			})
		}

		// Low cache hit rate recommendation
		if s.TotalBuilds >= 5 && s.CacheHitRate < 50 {
			recommendations = append(recommendations, model.BuildRecommendation{
				Type:               model.RecommendationTypeCaching,
				Priority:           model.RecommendationPriorityMedium,
				Title:              "Improve Docker layer caching",
				Description:        fmt.Sprintf("Resource '%s' has low cache hit rate (%.0f%%). Consider reordering Dockerfile instructions to maximize cache reuse.", s.ManifestName, s.CacheHitRate),
				AffectedResources:  []model.ManifestName{s.ManifestName},
				PotentialSavingsMs: s.AverageDurationMs / 3,
			})
		}

		// High variance recommendation
		if s.TotalBuilds >= 10 && s.MaxDurationMs > s.AverageDurationMs*3 {
			recommendations = append(recommendations, model.BuildRecommendation{
				Type:              model.RecommendationTypePerformance,
				Priority:          model.RecommendationPriorityLow,
				Title:             "Build time variance is high",
				Description:       fmt.Sprintf("Resource '%s' shows high variance in build times (max: %.1fs, avg: %.1fs). This may indicate intermittent issues or resource contention.", s.ManifestName, float64(s.MaxDurationMs)/1000, float64(s.AverageDurationMs)/1000),
				AffectedResources: []model.ManifestName{s.ManifestName},
			})
		}
	}

	// Sort by priority
	priorityOrder := map[model.RecommendationPriority]int{
		model.RecommendationPriorityHigh:   0,
		model.RecommendationPriorityMedium: 1,
		model.RecommendationPriorityLow:    2,
	}
	sort.Slice(recommendations, func(i, j int) bool {
		return priorityOrder[recommendations[i].Priority] < priorityOrder[recommendations[j].Priority]
	})

	return recommendations
}

// GetResourceStats returns statistics for a specific resource.
func (fs *FileStore) GetResourceStats(name model.ManifestName) (*model.ResourceStats, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	var metrics []model.BuildMetric
	for _, m := range fs.metrics {
		if m.ManifestName == name {
			metrics = append(metrics, m)
		}
	}

	if len(metrics) == 0 {
		return nil, fmt.Errorf("no metrics found for resource: %s", name)
	}

	stats := fs.computeResourceStats(name, metrics)
	return &stats, nil
}

// GetRecentBuilds returns the most recent n builds.
func (fs *FileStore) GetRecentBuilds(n int) ([]model.BuildMetric, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	return fs.getRecentBuildsLocked(n, fs.metrics), nil
}

// GetBuildHistory returns all builds for a resource within a time range.
func (fs *FileStore) GetBuildHistory(name model.ManifestName, since time.Time) ([]model.BuildMetric, error) {
	fs.mu.RLock()
	defer fs.mu.RUnlock()

	var results []model.BuildMetric
	for _, m := range fs.metrics {
		if m.ManifestName == name && (m.StartTime.After(since) || m.StartTime.Equal(since)) {
			results = append(results, m)
		}
	}

	// Sort by time descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].StartTime.After(results[j].StartTime)
	})

	return results, nil
}

// ClearOlderThan removes metrics older than the specified time.
func (fs *FileStore) ClearOlderThan(t time.Time) error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	var kept []model.BuildMetric
	for _, m := range fs.metrics {
		if m.StartTime.After(t) || m.StartTime.Equal(t) {
			kept = append(kept, m)
		}
	}

	if len(kept) != len(fs.metrics) {
		fs.metrics = kept
		fs.dirty = true
		return fs.flush()
	}

	return nil
}

// Close releases resources and flushes pending writes.
func (fs *FileStore) Close() error {
	fs.mu.Lock()
	defer fs.mu.Unlock()

	return fs.flush()
}

// SessionID returns the current session ID.
func (fs *FileStore) SessionID() string {
	return fs.sessionID
}

// Verify FileStore implements BuildInsightsStore.
var _ model.BuildInsightsStore = (*FileStore)(nil)
