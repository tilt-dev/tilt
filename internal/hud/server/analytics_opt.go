package server

import (
	"github.com/windmilleng/tilt/internal/hud/webview"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/wmclient/pkg/analytics"
)

type AnalyticsOpter interface {
	SetOptStr(s string) error
}

type AnalyticsOptAction struct {
	Opt analytics.Opt
}

func (AnalyticsOptAction) Action() {}

type WriteToFileOpter struct {
	st store.RStore
}

var _ AnalyticsOpter = &WriteToFileOpter{}

func ProvideAnalyticsOpter(st store.RStore) AnalyticsOpter {
	return &WriteToFileOpter{st: st}
}

func (o *WriteToFileOpter) SetOptStr(s string) error {
	if !webview.NewAnalyticsOn() {
		return nil
	}

	choice, err := analytics.SetOptStr(s)
	if err != nil {
		return err
	}
	o.st.Dispatch(AnalyticsOptAction{choice})
	return nil
}
