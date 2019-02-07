package engine

import (
	"context"
	"strconv"
	"time"

	"github.com/windmilleng/wmclient/pkg/analytics"

	"github.com/windmilleng/tilt/internal/store"
)

// How often to perodically report data for analytics while Tilt is running
const analyticsReportingInterval = time.Hour * 4

type AnalyticsReporter struct {
	a       analytics.Analytics
	store   *store.Store
	started bool
}

func (ar *AnalyticsReporter) OnChange(ctx context.Context, st store.RStore) {
	if ar.started {
		return
	}

	state := st.RLockState()
	defer st.RUnlockState()

	// wait until state has been kinda initialized
	if !state.TiltStartTime.IsZero() {
		ar.started = true
		go func() {
			ar.report()
			time.Sleep(analyticsReportingInterval)
		}()
	}
}

var _ store.Subscriber = &AnalyticsReporter{}

func ProvideAnalyticsReporter(a analytics.Analytics, st *store.Store) *AnalyticsReporter {
	return &AnalyticsReporter{a, st, false}
}

func (ar *AnalyticsReporter) report() {
	st := ar.store.RLockState()
	defer ar.store.RUnlockState()
	var dcCount, k8sCount, fastbuildCount int
	for _, m := range st.Manifests() {
		if m.IsK8s() {
			k8sCount++
		}
		if m.IsDC() {
			dcCount++
		}
		for _, it := range m.ImageTargets {
			if it.IsFastBuild() {
				fastbuildCount++
				break
			}
		}
	}

	ar.a.Incr("up.running", map[string]string{
		"up.starttime":                 st.TiltStartTime.Format(time.RFC3339),
		"builds.completed_count":       strconv.Itoa(st.CompletedBuildCount),
		"resource.count":               strconv.Itoa(len(st.ManifestDefinitionOrder)),
		"resource.dockercompose.count": strconv.Itoa(dcCount),
		"resource.k8s.count":           strconv.Itoa(k8sCount),
		"resource.fastbuild.count":     strconv.Itoa(fastbuildCount),
	})
}
