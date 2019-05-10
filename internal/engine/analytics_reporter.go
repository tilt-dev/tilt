package engine

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/wmclient/pkg/analytics"

	"github.com/windmilleng/tilt/internal/store"
)

// How often to periodically report data for analytics while Tilt is running
const analyticsReportingInterval = time.Hour * 1

const newAnalyticsFlag = "TILT_NEW_ANALYTICS"

// TEMPORARY: check env to see if new-analytics flag is set
func newAnalyticsOn() bool {
	return os.Getenv(newAnalyticsFlag) != ""
}

type AnalyticsReporter struct {
	a       analytics.Analytics
	store   *store.Store
	started bool
	opt     analytics.Opt
}

func (ar *AnalyticsReporter) OnChange(ctx context.Context, st store.RStore) {
	if ar.started {
		if newAnalyticsOn() {
			ar.maybeSetNeedsNudge()
		}
		return
	}

	state := st.RLockState()
	defer st.RUnlockState()

	// wait until state has been kinda initialized
	if !state.TiltStartTime.IsZero() && state.LastTiltfileError() == nil {
		ar.started = true

		// TODO(maia): make sure to update ar.opt when user opts in/out from new flow
		opt, err := analytics.OptStatus()
		if err != nil {
			logger.Get(ctx).Debugf("can't get analytics opt: %v", err)
		}
		ar.opt = opt

		go func() {
			for {
				select {
				case <-time.After(analyticsReportingInterval):
					ar.report()
				case <-ctx.Done():
					return
				}
			}
		}()
	}
}

var _ store.Subscriber = &AnalyticsReporter{}

func ProvideAnalyticsReporter(a analytics.Analytics, st *store.Store) *AnalyticsReporter {
	return &AnalyticsReporter{a: a, store: st, started: false}
}

func (ar *AnalyticsReporter) report() {
	st := ar.store.RLockState()
	defer ar.store.RUnlockState()
	var dcCount, k8sCount, fastbuildBaseCount, anyFastbuildCount, liveUpdateCount, unbuiltCount int
	for _, m := range st.Manifests() {
		if m.IsK8s() {
			k8sCount++
			if len(m.ImageTargets) == 0 {
				unbuiltCount++
			}
		}
		if m.IsDC() {
			dcCount++
		}
		for _, it := range m.ImageTargets {
			if !it.AnyFastBuildInfo().Empty() {
				anyFastbuildCount++
				if it.IsFastBuild() {
					fastbuildBaseCount++
				}
				break
			}
			if !it.AnyLiveUpdateInfo().Empty() {
				liveUpdateCount++
			}
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
	}

	stats["tiltfile.error"] = tiltfileIsInError

	ar.a.Incr("up.running", stats)
}

func (ar *AnalyticsReporter) maybeSetNeedsNudge() {
	if ar.needsNudge() {
		ar.store.Dispatch(NeedsAnalyticsNudgeAction{})
	}
}

// User needs nudge if:
// a. has not opted into or out of analytics
// b. at least one non-k8s manifest is (or has been) green
func (ar *AnalyticsReporter) needsNudge() bool {
	st := ar.store.RLockState()
	defer ar.store.RUnlockState()

	if st.NeedsAnalyticsNudge {
		return true
	}

	if ar.opt != analytics.OptDefault {
		// User has already made a choice
		return false
	}

	manifestTargs := st.ManifestTargets
	if len(manifestTargs) == 0 {
		return false
	}

	for _, targ := range manifestTargs {
		if targ.Manifest.IsUnresourcedYAMLManifest() {
			continue
		}

		if !targ.State.LastSuccessfulDeployTime.IsZero() {
			// At least one resource has at least one successful deploy
			return true
		}
	}
	return false
}
