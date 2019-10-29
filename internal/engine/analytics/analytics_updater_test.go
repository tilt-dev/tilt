package analytics

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/wmclient/pkg/analytics"

	tiltanalytics "github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/store"
)

func TestOnChange(t *testing.T) {
	to := &FakeOpter{}
	_, a := tiltanalytics.NewMemoryTiltAnalyticsForTest(to)
	cmdUpTags := CmdUpTags(map[string]string{"watch": "true"})
	au := NewAnalyticsUpdater(a, cmdUpTags)
	st, _ := store.NewStoreForTesting()
	setOpt(st, analytics.OptOut)
	au.OnChange(context.Background(), st)

	assert.Equal(t, []analytics.Opt{analytics.OptOut}, to.Calls())
}

func TestReportOnOptIn(t *testing.T) {
	to := &FakeOpter{}
	mem, a := tiltanalytics.NewMemoryTiltAnalyticsForTest(to)
	a.SetOpt(analytics.OptDefault)

	cmdUpTags := CmdUpTags(map[string]string{"watch": "true"})
	au := NewAnalyticsUpdater(a, cmdUpTags)
	st, _ := store.NewStoreForTesting()
	setOpt(st, analytics.OptIn)
	au.OnChange(context.Background(), st)

	assert.Equal(t, []analytics.Opt{analytics.OptDefault, analytics.OptIn}, to.Calls())
	if assert.Equal(t, 1, len(mem.Counts)) {
		assert.Equal(t, "cmd.up", mem.Counts[0].Name)
		assert.Equal(t, "true", mem.Counts[0].Tags["watch"])
	}

	// opt-out then back in again, and make sure it doesn't get re-reported.
	setOpt(st, analytics.OptOut)
	au.OnChange(context.Background(), st)

	setOpt(st, analytics.OptIn)
	au.OnChange(context.Background(), st)
	assert.Equal(t, 1, len(mem.Counts))
}

func setOpt(st *store.Store, opt analytics.Opt) {
	state := st.LockMutableStateForTesting()
	defer st.UnlockMutableState()
	state.AnalyticsOpt = opt
}
