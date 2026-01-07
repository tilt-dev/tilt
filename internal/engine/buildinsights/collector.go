package buildinsights

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Collector is a store subscriber that collects build metrics.
// It listens for BuildCompleteAction events and records them to the insights store.
type Collector struct {
	store      model.BuildInsightsStore
	lastBuilds map[model.ManifestName]time.Time
}

// NewCollector creates a new build insights collector.
func NewCollector(store model.BuildInsightsStore) *Collector {
	return &Collector{
		store:      store,
		lastBuilds: make(map[model.ManifestName]time.Time),
	}
}

// OnChange implements store.Subscriber.
// It checks for build completion events and records metrics.
func (c *Collector) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) error {
	state := st.RLockState()
	defer st.RUnlockState()

	// Check each manifest for completed builds
	for _, mt := range state.ManifestTargets {
		if mt == nil || mt.State == nil {
			continue
		}

		ms := mt.State

		// Get the most recent completed build
		if len(ms.BuildHistory) == 0 {
			continue
		}

		lastBuild := ms.BuildHistory[0]
		if lastBuild.FinishTime.IsZero() {
			continue
		}

		// Check if we've already recorded this build
		if recorded, ok := c.lastBuilds[mt.Manifest.Name]; ok {
			if !lastBuild.FinishTime.After(recorded) {
				continue
			}
		}

		// Record this build
		metric := c.buildRecordToMetric(mt.Manifest.Name, lastBuild)
		if err := c.store.RecordBuild(metric); err != nil {
			logger.Get(ctx).Debugf("Failed to record build metric: %v", err)
			continue
		}

		c.lastBuilds[mt.Manifest.Name] = lastBuild.FinishTime
	}

	return nil
}

// buildRecordToMetric converts a BuildRecord to a BuildMetric.
func (c *Collector) buildRecordToMetric(name model.ManifestName, br model.BuildRecord) model.BuildMetric {
	metric := model.BuildMetric{
		BuildID:      uuid.New().String(),
		ManifestName: name,
		BuildTypes:   br.BuildTypes,
		StartTime:    br.StartTime,
		FinishTime:   br.FinishTime,
		DurationMs:   br.Duration().Milliseconds(),
		Success:      br.Error == nil,
		WarningCount: br.WarningCount,
		Reason:       br.Reason,
		FilesChanged: len(br.Edits),
	}

	if br.Error != nil {
		metric.ErrorMessage = br.Error.Error()
	}

	// Determine if this was a live update
	for _, bt := range br.BuildTypes {
		if bt == model.BuildTypeLiveUpdate {
			metric.LiveUpdate = true
			break
		}
	}

	return metric
}

// Close cleans up resources.
func (c *Collector) Close() error {
	if c.store != nil {
		return c.store.Close()
	}
	return nil
}

// Store returns the underlying insights store.
func (c *Collector) Store() model.BuildInsightsStore {
	return c.store
}

// Verify Collector implements store.Subscriber.
var _ store.Subscriber = (*Collector)(nil)
