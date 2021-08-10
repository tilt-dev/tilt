package analytics

import (
	"context"

	wmanalytics "github.com/tilt-dev/wmclient/pkg/analytics"

	"github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
)

type CmdTags map[string]string

func (t CmdTags) AsMap() map[string]string {
	return (map[string]string)(t)
}

type AnalyticsUpdater struct {
	ta          *analytics.TiltAnalytics
	cmdTags     CmdTags
	reportedCmd bool
	engineMode  store.EngineMode
}

func NewAnalyticsUpdater(ta *analytics.TiltAnalytics, cmdTags CmdTags, engineMode store.EngineMode) *AnalyticsUpdater {
	return &AnalyticsUpdater{
		ta:          ta,
		cmdTags:     cmdTags,
		reportedCmd: ta.EffectiveOpt() != wmanalytics.OptOut,
		engineMode:  engineMode,
	}
}

func (sub *AnalyticsUpdater) OnChange(ctx context.Context, st store.RStore, _ store.ChangeSummary) error {
	state := st.RLockState()
	defer st.RUnlockState()

	sub.ta.SetTiltfileOpt(state.AnalyticsTiltfileOpt)
	err := sub.ta.SetUserOpt(state.AnalyticsUserOpt)
	if err != nil {
		logger.Get(ctx).Infof("error saving analytics opt (tried to record opt: '%s')", state.AnalyticsUserOpt)
	}

	if sub.ta.EffectiveOpt() != wmanalytics.OptOut && !sub.reportedCmd {
		sub.reportedCmd = true

		cmd := "cmd.up"
		if sub.engineMode.IsCIMode() {
			cmd = "cmd.ci"
		}

		sub.ta.Incr(cmd, sub.cmdTags.AsMap())
	}

	return nil
}

var _ store.Subscriber = &AnalyticsUpdater{}
