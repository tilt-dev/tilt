package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/store"
)

type TiltAnalyticsSubscriber struct {
	ta *analytics.TiltAnalytics
}

func NewTiltAnalyticsSubscriber(ta *analytics.TiltAnalytics) *TiltAnalyticsSubscriber {
	return &TiltAnalyticsSubscriber{ta: ta}
}

func (sub *TiltAnalyticsSubscriber) OnChange(ctx context.Context, st store.RStore) {
	state := st.RLockState()
	defer st.RUnlockState()
	if state.AnalyticsOpt != sub.ta.Opt() {
		err := sub.ta.SetOpt(state.AnalyticsOpt)
		if err != nil {
			logger.Get(ctx).Infof("error saving analytics opt (tried to record opt: '%s')", state.AnalyticsOpt)
		}
	}
}

var _ store.Subscriber = &TiltAnalyticsSubscriber{}
