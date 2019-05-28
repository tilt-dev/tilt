package engine

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/wmclient/pkg/analytics"

	tiltanalytics "github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/store"
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

	assert.Equal(t, []analytics.Opt{analytics.OptOut}, to.Calls())
}
