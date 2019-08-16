package engine

import (
	"context"
	"strconv"
	"time"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/store"
)

// How often to periodically report data for analytics while Tilt is running
const analyticsReportingInterval = time.Hour * 1

type AnalyticsReporter struct {
	a       *analytics.TiltAnalytics
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
	if !state.TiltStartTime.IsZero() && state.LastTiltfileError() == nil {
		ar.started = true
		ar.report() // report once now...
		go func() {
			for {
				select {
				case <-time.After(analyticsReportingInterval):
					// and once every <interval> thereafter
					ar.report()
				case <-ctx.Done():
					return
				}
			}
		}()
	}
}

var _ store.Subscriber = &AnalyticsReporter{}

func ProvideAnalyticsReporter(a *analytics.TiltAnalytics, st *store.Store) *AnalyticsReporter {
	return &AnalyticsReporter{a, st, false}
}

func (ar *AnalyticsReporter) report() {
	st := ar.store.RLockState()
	defer ar.store.RUnlockState()
	var dcCount, k8sCount, fastbuildBaseCount, anyFastbuildCount, liveUpdateCount,
		unbuiltCount, sameImgMultiContainerLiveUpdate, multiImgLiveUpdate int
	for _, m := range st.Manifests() {
		var refInjectCounts map[string]int
		if m.IsK8s() {
			k8sCount++
			refInjectCounts = m.K8sTarget().RefInjectCounts()
			if len(m.ImageTargets) == 0 {
				unbuiltCount++
			}
		}
		if m.IsDC() {
			dcCount++
		}
		var seenLU, multiImgLU, multiContainerLU bool
		for _, it := range m.ImageTargets {
			if !it.AnyFastBuildInfo().Empty() {
				anyFastbuildCount++
				if it.IsFastBuild() {
					fastbuildBaseCount++
				}
				break
			}
			if !it.AnyLiveUpdateInfo().Empty() {
				if !seenLU {
					seenLU = true
					liveUpdateCount++
				} else if !multiImgLU {
					multiImgLU = true
				}
				multiContainerLU = multiContainerLU ||
					refInjectCounts[it.ConfigurationRef.String()] > 0
			}
		}
		if multiContainerLU {
			sameImgMultiContainerLiveUpdate++
		}
		if multiImgLU {
			multiImgLiveUpdate++
		}
	}

	stats := map[string]string{
		"up.starttime":           st.TiltStartTime.Format(time.RFC3339),
		"builds.completed_count": strconv.Itoa(st.CompletedBuildCount),
	}

	tiltfileIsInError := "false"
	if st.LastTiltfileError() != nil {
		tiltfileIsInError = "true"
	} else {
		// only report when there's no tiltfile error, to avoid polluting aggregations
		stats["resource.count"] = strconv.Itoa(len(st.ManifestDefinitionOrder))
		stats["resource.dockercompose.count"] = strconv.Itoa(dcCount)
		stats["resource.k8s.count"] = strconv.Itoa(k8sCount)
		stats["resource.fastbuild.count"] = strconv.Itoa(fastbuildBaseCount)
		stats["resource.anyfastbuild.count"] = strconv.Itoa(anyFastbuildCount)
		stats["resource.liveupdate.count"] = strconv.Itoa(liveUpdateCount)
		stats["resource.unbuiltresources.count"] = strconv.Itoa(unbuiltCount)
		stats["resource.sameimagemultiplecontainerliveupdate.count"] = strconv.Itoa(sameImgMultiContainerLiveUpdate)
		stats["resource.multipleimageliveupdate.count"] = strconv.Itoa(multiImgLiveUpdate)
	}

	stats["tiltfile.error"] = tiltfileIsInError

	ar.a.Incr("up.running", stats)
}
