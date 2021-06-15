package metrics

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestEnableMetrics(t *testing.T) {
	f := newFixture(t)

	assert.Nil(t, f.exp.remote)

	// Verify that enabling metrics creates a remote exporter.
	ms := model.DefaultMetricsSettings()
	ms.Enabled = true
	f.st.SetState(newLoggedInEngineState(ms))
	_ = f.mc.OnChange(f.ctx, f.st, store.LegacyChangeSummary())

	remote := f.exp.remote
	assert.NotNil(t, remote)

	_ = f.mc.OnChange(f.ctx, f.st, store.LegacyChangeSummary())
	assert.Same(t, remote, f.exp.remote)

	// Verify that changing the metrics settings creates a new remote exporter.
	ms.Insecure = true
	f.st.SetState(newLoggedInEngineState(ms))
	_ = f.mc.OnChange(f.ctx, f.st, store.LegacyChangeSummary())
	assert.NotSame(t, remote, f.exp.remote)

	// Verify that disabling the metrics settings nulls out the remote exporter.
	ms.Enabled = false
	f.st.SetState(newLoggedInEngineState(ms))
	_ = f.mc.OnChange(f.ctx, f.st, store.LegacyChangeSummary())
	assert.Nil(t, f.exp.remote)
}

type fixture struct {
	*tempdir.TempDirFixture
	ctx context.Context
	st  *store.TestingStore
	exp *DeferredExporter
	mc  *Controller
}

func newFixture(t *testing.T) *fixture {
	f := tempdir.NewTempDirFixture(t)

	st := store.NewTestingStore()

	ctx := logger.WithLogger(context.Background(), logger.NewTestLogger(os.Stdout))

	exp := NewDeferredExporter()
	mc := NewController(exp, model.TiltBuild{}, "")
	return &fixture{
		TempDirFixture: f,
		ctx:            ctx,
		st:             st,
		exp:            exp,
		mc:             mc,
	}
}

func newLoggedInEngineState(ms model.MetricsSettings) store.EngineState {
	return store.EngineState{
		Token:  "x",
		TeamID: "bar",
		CloudStatus: store.CloudStatus{
			Username: "nicks",
		},
		MetricsSettings: ms,
	}
}
