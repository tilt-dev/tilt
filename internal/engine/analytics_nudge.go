package engine

import (
	"os"

	"github.com/windmilleng/tilt/internal/store"
)

const newAnalyticsFlag = "TILT_NEW_ANALYTICS"

// TEMPORARY: check env to see if new-analytics flag is set
func newAnalyticsOn() bool {
	return os.Getenv(newAnalyticsFlag) != ""
}

func MaybeSetNeedsNudge(mt *store.ManifestTarget, st *store.EngineState) {
	if !newAnalyticsOn() {
		return
	}
	maybeSetNeedsNudge(mt, st)
}

func maybeSetNeedsNudge(mt *store.ManifestTarget, st *store.EngineState) {
	if st.NeedsAnalyticsNudge {
		// already set
		return
	}

	if !mt.Manifest.IsUnresourcedYAMLManifest() && !mt.State.LastSuccessfulDeployTime.IsZero() {
		// there exists at least one non-k8s_yaml manifest that is/has been green!
		st.NeedsAnalyticsNudge = true
	}
}
