package webview

import (
	"github.com/tilt-dev/wmclient/pkg/analytics"

	"github.com/tilt-dev/tilt/internal/store"
)

func NeedsNudge(st store.EngineState) bool {
	if st.AnalyticsEffectiveOpt() != analytics.OptDefault {
		// already opted in/out
		return false
	}

	manifestTargs := st.ManifestTargets
	if len(manifestTargs) == 0 {
		return false
	}

	for _, targ := range manifestTargs {
		if targ.Manifest.NonWorkloadManifest() {
			continue
		}

		if !targ.State.LastSuccessfulDeployTime.IsZero() {
			// A resource has been green at some point
			return true
		}
	}
	return false
}
