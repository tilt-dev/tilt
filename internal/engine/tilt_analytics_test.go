package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	tiltanalytics "github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/wmclient/pkg/analytics"
)

func TestOnChange(t *testing.T) {
	to := &testOpter{}
	_, a := tiltanalytics.NewMemoryTiltAnalyticsForTest(to)
	tas := NewTiltAnalyticsSubscriber(a)
	st, _ := store.NewStoreForTesting()
	state := st.LockMutableStateForTesting()
	state.AnalyticsOpt = analytics.OptOut
	st.UnlockMutableState()
	ctx := context.Background()
	tas.OnChange(ctx, st)

	assert.Equal(t, []analytics.Opt{analytics.OptOut}, to.calls)
}
