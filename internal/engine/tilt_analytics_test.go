package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/wmclient/pkg/analytics"

	"github.com/windmilleng/tilt/internal/testutils"

	tiltanalytics "github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/store"
)

func TestOnChange(t *testing.T) {
	to := &testOpter{}
	ctx := testutils.CtxForTest()
	ctx, _, a := tiltanalytics.NewMemoryTiltAnalyticsForTest(ctx, to)
	tas := NewTiltAnalyticsSubscriber(a)
	st, _ := store.NewStoreForTesting()
	state := st.LockMutableStateForTesting()
	state.AnalyticsOpt = analytics.OptOut
	st.UnlockMutableState()
	tas.OnChange(ctx, st)

	assert.Equal(t, []analytics.Opt{analytics.OptOut}, to.Calls())
}
