package analytics

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stretchr/testify/assert"
	"github.com/tilt-dev/wmclient/pkg/analytics"

	tiltanalytics "github.com/tilt-dev/tilt/internal/analytics"
	"github.com/tilt-dev/tilt/internal/store"
)

func TestOnChange(t *testing.T) {
	to := tiltanalytics.NewFakeOpter(analytics.OptIn)
	_, a := tiltanalytics.NewMemoryTiltAnalyticsForTest(to)
	cmdUpTags := CmdTags(map[string]string{"watch": "true"})
	au := NewAnalyticsUpdater(a, cmdUpTags)
	st := store.NewTestingStore()
	setUserOpt(st, analytics.OptOut)
	au.OnChange(context.Background(), st)

	assert.Equal(t, []analytics.Opt{analytics.OptOut}, to.Calls())
}

func TestReportOnOptIn(t *testing.T) {
	to := tiltanalytics.NewFakeOpter(analytics.OptIn)
	mem, a := tiltanalytics.NewMemoryTiltAnalyticsForTest(to)
	err := a.SetUserOpt(analytics.OptOut)
	require.NoError(t, err)

	cmdUpTags := CmdTags(map[string]string{"watch": "true"})
	au := NewAnalyticsUpdater(a, cmdUpTags)
	st := store.NewTestingStore()
	setUserOpt(st, analytics.OptIn)
	au.OnChange(context.Background(), st)

	assert.Equal(t, []analytics.Opt{analytics.OptOut, analytics.OptIn}, to.Calls())
	if assert.Equal(t, 1, len(mem.Counts)) {
		assert.Equal(t, "cmd.up", mem.Counts[0].Name)
		assert.Equal(t, "true", mem.Counts[0].Tags["watch"])
	}

	// opt-out then back in again, and make sure it doesn't get re-reported.
	setUserOpt(st, analytics.OptOut)
	au.OnChange(context.Background(), st)

	setUserOpt(st, analytics.OptIn)
	au.OnChange(context.Background(), st)
	assert.Equal(t, 1, len(mem.Counts))
}

func setUserOpt(st *store.TestingStore, opt analytics.Opt) {
	state := st.LockMutableStateForTesting()
	defer st.UnlockMutableState()
	state.AnalyticsUserOpt = opt
}
