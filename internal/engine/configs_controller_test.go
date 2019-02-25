package engine

import (
	"context"
	"os"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/wmclient/pkg/analytics"

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

	f.tfl.BuiltinCallCounts = map[string]int{
		"docker_build": 10,
		"custom_build": 2,
	}

	a := f.run()

	expectedAction := ConfigsReloadedAction{
		Manifests:  []model.Manifest{{Name: "bar"}},
		StartTime:  f.fc.Times[0],
		FinishTime: f.fc.Times[1],
	}

	assert.Equal(t, expectedAction, a)

	expectedCounts := []analytics.CountEvent{{
		Name: "tiltfile.loaded",
		Tags: map[string]string{
			"tiltfile.invoked.docker_build": "10",
			"tiltfile.invoked.custom_build": "2",
		},
		N: 1,
	}}

	assert.Equal(t, expectedCounts, f.an.Counts)
}

func (f *ccFixture) run() ConfigsReloadedAction {
	// configs_controller uses state.RelativeTiltfilePath, which is relative to wd
	// sometimes the original directory was invalid (e.g., it was another test's temp dir, which was deleted,
	// but not changed out of), and if it was already invalid, then let's not worry about it.
	origDir, _ := os.Getwd()
	err := os.Chdir(f.Path())
	if err != nil {
		f.T().Fatalf("error changing dir: %v", err)
	}
	defer func() {
		if origDir != "" {
			_ = os.Chdir(origDir)
		}
	}()

	f.tfl.Manifests = []model.Manifest{{Name: "bar"}}

	f.st.NotifySubscribers(f.ctx)

	a := waitForAction(f.T(), reflect.TypeOf(ConfigsReloadedAction{}), f.getActions)
	cra, ok := a.(ConfigsReloadedAction)
	if !ok {
		f.T().Fatalf("didn't get an action of type %T", ConfigsReloadedAction{})
	}

	return cra
}

type ccFixture struct {
	*tempdir.TempDirFixture
	ctx        context.Context
	cc         *ConfigsController
	st         *store.Store
	getActions func() []store.Action
	tfl        *tiltfile.FakeTiltfileLoader
	fc         *testutils.FakeClock
	an         *analytics.MemoryAnalytics
}

func newCCFixture(t *testing.T) *ccFixture {
	f := tempdir.NewTempDirFixture(t)
	st, getActions := store.NewStoreForTesting()
	tfl := tiltfile.NewFakeTiltfileLoader()
	an := analytics.NewMemoryAnalytics()
	cc := NewConfigsController(tfl, an)
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
		getActions:     getActions,
		tfl:            tfl,
		fc:             fc,
		an:             an,
	}
}
