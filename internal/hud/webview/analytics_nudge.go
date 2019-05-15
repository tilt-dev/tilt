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

type Opter interface {
	// OptStatus() (analytics.Opt, error)
	SetOptStr(s string) (analytics.Opt, error)
}

type AnalyticsOptAction struct {
	Opt analytics.Opt
}

func (AnalyticsOptAction) Action() {}

type WriteToFileOpter struct {
	st store.RStore
}

var _ Opter = &WriteToFileOpter{}

func (*WriteToFileOpter) OptStatus() (analytics.Opt, error) {
	return analytics.OptStatus()
}

func (o *WriteToFileOpter) SetOptStr(s string) (analytics.Opt, error) {
	if !NewAnalyticsOn() {
		return analytics.OptDefault, nil
	}

	choice, err := analytics.SetOptStr(s)
	if err != nil {
		return choice, err
	}
	o.st.Dispatch(AnalyticsOptAction{choice})
	return choice, nil
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
