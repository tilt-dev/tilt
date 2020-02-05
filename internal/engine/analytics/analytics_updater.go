package analytics

import (
	"context"

	wmanalytics "github.com/windmilleng/wmclient/pkg/analytics"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
)

type CmdUpTags map[string]string

func (t CmdUpTags) AsMap() map[string]string {
	return (map[string]string)(t)
}

type AnalyticsUpdater struct {
	ta            *analytics.TiltAnalytics
	cmdUpTags     CmdUpTags
	reportedCmdUp bool
}

func NewAnalyticsUpdater(ta *analytics.TiltAnalytics, cmdUpTags CmdUpTags) *AnalyticsUpdater {
	return &AnalyticsUpdater{
		ta:            ta,
		cmdUpTags:     cmdUpTags,
		reportedCmdUp: ta.EffectiveOpt() != wmanalytics.OptOut,
	}
}

func (sub *AnalyticsUpdater) OnChange(ctx context.Context, st store.RStore) {
	state := st.RLockState()
	defer st.RUnlockState()

	sub.ta.SetTiltfileOpt(state.AnalyticsTiltfileOpt)
	err := sub.ta.SetUserOpt(state.AnalyticsUserOpt)
	if err != nil {
		logger.Get(ctx).Infof("error saving analytics opt (tried to record opt: '%s')", state.AnalyticsUserOpt)
	}

	if sub.ta.EffectiveOpt() != wmanalytics.OptOut && !sub.reportedCmdUp {
		sub.reportedCmdUp = true
		sub.ta.Incr("cmd.up", sub.cmdUpTags.AsMap())
	}
}

var _ store.Subscriber = &AnalyticsUpdater{}
