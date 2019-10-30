package analytics

import (
	"context"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
)

type AnalyticsUpdater struct {
	ta *analytics.TiltAnalytics
}

func NewAnalyticsUpdater(ta *analytics.TiltAnalytics) *AnalyticsUpdater {
	return &AnalyticsUpdater{ta: ta}
}

func (sub *AnalyticsUpdater) OnChange(ctx context.Context, st store.RStore) {
	state := st.RLockState()
	defer st.RUnlockState()
	if state.AnalyticsOpt != sub.ta.Opt() {
		err := sub.ta.SetOpt(state.AnalyticsOpt)
		if err != nil {
			logger.Get(ctx).Infof("error saving analytics opt (tried to record opt: '%s')", state.AnalyticsOpt)
		}
	}
}

var _ store.Subscriber = &AnalyticsUpdater{}
