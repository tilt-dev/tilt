package metrics

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/internal/user"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestEnableLocalMetrics(t *testing.T) {
	f := newModeFixture(t)
	os.Setenv("TILT_METRICS", "local")
	defer os.Unsetenv("TILT_METRICS")

	f.mc.OnChange(f.ctx, f.st)
	if assert.NotNil(t, f.st.action) {
		assert.Equal(t, model.MetricsLocal, f.st.action.Serving.Mode)
		assert.Equal(t, 3, len(f.st.action.Manifests))
	}
}

type modeStore struct {
	*store.TestingStore

	action MetricsModeAction
}

func (s *modeStore) Dispatch(action store.Action) {
	mma, ok := action.(MetricsModeAction)
	if ok {
		s.action = mma
	}
	s.TestingStore.Dispatch(mma)
}

type modeFixture struct {
	*tempdir.TempDirFixture
	ctx context.Context
	st  *modeStore
	mc  *ModeController
}

func newModeFixture(t *testing.T) *modeFixture {
	f := tempdir.NewTempDirFixture(t)

	st := &modeStore{TestingStore: store.NewTestingStore()}

	l := logger.NewLogger(logger.DebugLvl, os.Stdout)
	ctx := logger.WithLogger(context.Background(), l)

	mc := NewModeController("localhost", user.NewFakePrefs())
	return &modeFixture{
		TempDirFixture: f,
		ctx:            ctx,
		st:             st,
		mc:             mc,
	}
}
