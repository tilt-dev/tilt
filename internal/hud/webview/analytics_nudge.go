package webview

import (
	"os"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/wmclient/pkg/analytics"
)

const newAnalyticsFlag = "TILT_NEW_ANALYTICS"

// TEMPORARY: check env to see if new-analytics flag is set
func NewAnalyticsOn() bool {
	return os.Getenv(newAnalyticsFlag) != ""
}

func NeedsNudge(st store.EngineState) bool {
	if !NewAnalyticsOn() {
		return false
	}
	return needsNudge(st)
}

func needsNudge(st store.EngineState) bool {
	if st.AnalyticsOpt != analytics.OptDefault {
		// already opted in/out
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
			// A resource has been green at some point
			return true
		}
	}
	return false
}
