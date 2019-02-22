package engine

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/internal/tiltfile"
)

func TestConfigsController(t *testing.T) {
	f := newCCFixture(t)
	defer f.TearDown()

	state := f.st.LockMutableStateForTesting()
	m := model.Manifest{Name: "foo"}
	mt := store.NewManifestTarget(m)
	state.UpsertManifestTarget(mt)
	state.PendingConfigFileChanges["Tiltfile"] = true
	state.TiltfilePath = f.JoinPath("Tiltfile")
	f.st.UnlockMutableState()

	a := f.run()

	expected := ConfigsReloadedAction{
		Manifests:  []model.Manifest{{Name: "bar"}},
		StartTime:  f.fc.Times[0],
		FinishTime: f.fc.Times[1],
	}

	assert.Equal(t, expected, a)
}

func (f *ccFixture) run() ConfigsReloadedAction {
	// configs_controller uses state.RelativeTiltfilePath, which is relative to wd
	origDir, err := os.Getwd()
	if err != nil {
		f.T().Fatalf("error getting wd: %v", err)
	}
	err = os.Chdir(f.Path())
	if err != nil {
		f.T().Fatalf("error changing dir: %v", err)
	}
	defer func() {
		// sometimes the original directory was invalid (e.g., it was another test's temp dir, which was deleted,
		// but not changed out of), so changing back to the original directory will fail, and we probably don't care.
		_ = os.Chdir(origDir)
	}()

	f.tfl.Manifests = []model.Manifest{{Name: "bar"}}

	f.st.NotifySubscribers(f.ctx)

	a := waitForAction(f.T(), reflect.TypeOf(ConfigsReloadedAction{}), f.actions)
	cra, ok := a.(ConfigsReloadedAction)
	if !ok {
		f.T().Fatalf("didn't get an action of type %T", ConfigsReloadedAction{})
	}

	return cra
}

type ccFixture struct {
	*tempdir.TempDirFixture
	ctx     context.Context
	cc      *ConfigsController
	st      *store.Store
	actions <-chan store.Action
	tfl     *tiltfile.FakeTiltfileLoader
	fc      *testutils.FakeClock
}

func newCCFixture(t *testing.T) *ccFixture {
	f := tempdir.NewTempDirFixture(t)
	st, actions := store.NewStoreForTesting()
	tfl := tiltfile.NewFakeTiltfileLoader()
	cc := NewConfigsController(tfl)
	fc := testutils.NewRandomFakeClock()
	cc.clock = fc.Clock()
	ctx := output.CtxForTest()
	st.AddSubscriber(cc)
	go st.Loop(ctx)
	return &ccFixture{
		TempDirFixture: f,
		ctx:            ctx,
		cc:             cc,
		st:             st,
		actions:        actions,
		tfl:            tfl,
		fc:             fc,
	}
}
