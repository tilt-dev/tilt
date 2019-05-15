package server

import (
	"github.com/windmilleng/tilt/internal/hud/webview"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/wmclient/pkg/analytics"
)

// An AnalyticsOpter can record a user's choice (opt-in or opt-out)
// in re: Tilt recording analytics.
type AnalyticsOpter interface {
	SetOptStr(s string) error
}

type WriteToFileOpter struct {
	st store.RStore
}

var _ AnalyticsOpter = &WriteToFileOpter{}

func ProvideAnalyticsOpter(st store.RStore) AnalyticsOpter {
	return &WriteToFileOpter{st: st}
}

// SetOptStr takes the string of the user's choice (should correspond to "opt-in" or "opt-out")
// and records that choice on disk as dictated by the `analytics` package.
func (o *WriteToFileOpter) SetOptStr(s string) error {
	if !webview.NewAnalyticsOn() {
		return nil
	}

	choice, err := analytics.SetOptStr(s)
	if err != nil {
		return err
	}
	o.st.Dispatch(store.AnalyticsOptAction{Opt: choice})
	return nil
}
