package analytics

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/controllers/apis/liveupdate"
	"github.com/tilt-dev/tilt/internal/feature"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// How often to periodically report data for analytics while Tilt is running
const analyticsReportingInterval = time.Minute * 15

type AnalyticsReporter struct {
	a               *analytics.TiltAnalytics
	store           store.RStore
	kClient         k8s.Client
	env             k8s.Env
	featureDefaults feature.Defaults
	started         bool
}

func (ar *AnalyticsReporter) OnChange(ctx context.Context, st store.RStore, _ store.ChangeSummary) error {
	if ar.started {
		return nil
	}

	state := st.RLockState()
	defer st.RUnlockState()

	// wait until state has been kinda initialized
	if !state.TiltStartTime.IsZero() && state.LastMainTiltfileError() == nil {
		ar.started = true
		go func() {
			select {
			case <-time.After(10 * time.Second):
				ar.report(ctx) // report once pretty soon after startup...
			case <-ctx.Done():
				return
			}

			for {
				select {
				case <-time.After(analyticsReportingInterval):
					// and once every <interval> thereafter
					ar.report(ctx)
				case <-ctx.Done():
					return
				}
			}
		}()
	}

	return nil
}

var _ store.Subscriber = &AnalyticsReporter{}

func ProvideAnalyticsReporter(
	a *analytics.TiltAnalytics,
	st store.RStore,
	kClient k8s.Client,
	env k8s.Env,
	fDefaults feature.Defaults) *AnalyticsReporter {
	return &AnalyticsReporter{
		a:               a,
		store:           st,
		kClient:         kClient,
		env:             env,
		featureDefaults: fDefaults,
		started:         false,
	}
}

func (ar *AnalyticsReporter) report(ctx context.Context) {
	st := ar.store.RLockState()
	defer ar.store.RUnlockState()
	var dcCount, k8sCount, liveUpdateCount, unbuiltCount,
		sameImgMultiContainerLiveUpdate, multiImgLiveUpdate,
		localCount, localServeCount, enabledCount int

	labelKeySet := make(map[string]bool)

	for _, mt := range st.ManifestTargets {
		m := mt.Manifest
		for key := range m.Labels {
			labelKeySet[key] = true
		}

		if mt.State.DisableState == v1alpha1.DisableStateEnabled {
			enabledCount++
		}

		if m.IsLocal() {
			localCount++

			lt := m.LocalTarget()
			if !lt.ServeCmd.Empty() {
				localServeCount++
			}
		}

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
			if !liveupdate.IsEmptySpec(it.LiveUpdateSpec) {
				if !seenLU {
					seenLU = true
					liveUpdateCount++
				} else if !multiImgLU {
					multiImgLU = true
				}
				multiContainerLU = multiContainerLU ||
					refInjectCounts[it.Refs.ConfigurationRef.String()] > 0
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

		// env should really be a global tag, but there's a circular dependency
		// between the global tags and env initialization, so we add it manually.
		"env": string(ar.env),

		"term_mode": strconv.Itoa(int(st.TerminalMode)),
	}

	if k8sCount > 1 {
		registry := ar.kClient.LocalRegistry(ctx)
		if registry.Host != "" {
			stats["k8s.registry.host"] = "1"
		}
		if registry.HostFromCluster() != registry.Host {
			stats["k8s.registry.hostFromCluster"] = "1"
		}

		stats["k8s.runtime"] = string(ar.kClient.ContainerRuntime(ctx))
	}

	tiltfileIsInError := "false"
	if st.LastMainTiltfileError() != nil {
		tiltfileIsInError = "true"
	} else {
		// only report when there's no tiltfile error, to avoid polluting aggregations
		stats["resource.count"] = strconv.Itoa(len(st.ManifestDefinitionOrder))
		stats["resource.local.count"] = strconv.Itoa(localCount)
		stats["resource.localserve.count"] = strconv.Itoa(localServeCount)
		stats["resource.dockercompose.count"] = strconv.Itoa(dcCount)
		stats["resource.k8s.count"] = strconv.Itoa(k8sCount)
		stats["resource.liveupdate.count"] = strconv.Itoa(liveUpdateCount)
		stats["resource.unbuiltresources.count"] = strconv.Itoa(unbuiltCount)
		stats["resource.sameimagemultiplecontainerliveupdate.count"] = strconv.Itoa(sameImgMultiContainerLiveUpdate)
		stats["resource.multipleimageliveupdate.count"] = strconv.Itoa(multiImgLiveUpdate)
		stats["label.count"] = strconv.Itoa(len(labelKeySet))
		stats["resource.enabled.count"] = strconv.Itoa(enabledCount)
	}

	stats["tiltfile.error"] = tiltfileIsInError

	for k, v := range st.Features {
		if ar.featureDefaults[k].Status == feature.Active && v {
			stats[fmt.Sprintf("feature.%s.enabled", k)] = strconv.FormatBool(v)
		}
	}

	ar.a.Incr("up.running", stats)
}
