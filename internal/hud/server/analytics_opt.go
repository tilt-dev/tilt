package server

import (
	"github.com/windmilleng/wmclient/pkg/analytics"

	tiltanalytics "github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/hud/webview"
	"github.com/windmilleng/tilt/internal/store"
)

type WriteToFileOpter struct {
	st store.RStore
}

var _ tiltanalytics.AnalyticsOpter = &WriteToFileOpter{}

func ProvideAnalyticsOpter(st store.RStore) tiltanalytics.AnalyticsOpter {
	return &WriteToFileOpter{st: st}
}

// SetOptStr takes the string of the user's choice in re: sending analytics
// (should correspond to "opt-in" or "opt-out") and records that choice on
// disk as dictated by the `analytics` package.
func (o *WriteToFileOpter) SetOpt(opt analytics.Opt) error {
	if !webview.NewAnalyticsOn() {
		return nil
	}

	err := analytics.SetOpt(opt)
	if err != nil {
		return err
	}
	o.st.Dispatch(store.AnalyticsOptAction{Opt: opt})
	return nil
}
