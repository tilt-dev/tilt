package configs

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/docker"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/testutils/manifestbuilder"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
	"github.com/windmilleng/tilt/internal/tiltfile"
	"github.com/windmilleng/tilt/pkg/model"
)

func TestConfigsController(t *testing.T) {
	f := newCCFixture(t)
	defer f.TearDown()

	assert.Equal(t, model.OrchestratorUnknown, f.docker.Orchestrator)
	f.addManifest("fe")

	bar := manifestbuilder.New(f, "bar").WithK8sYAML(testyaml.SanchoYAML).Build()
	a := f.run(bar)

	expected := ConfigsReloadedAction{
		Manifests:  []model.Manifest{bar},
		FinishTime: f.fc.Times[1],
	}

	assert.Equal(t, expected, a)
	assert.Equal(t, model.OrchestratorK8s, f.docker.Orchestrator)
}

func TestConfigsControllerDockerNotConnected(t *testing.T) {
	f := newCCFixture(t)
	defer f.TearDown()

	assert.Equal(t, model.OrchestratorUnknown, f.docker.Orchestrator)
	f.addManifest("fe")
	f.docker.CheckConnectedErr = fmt.Errorf("connection-error")

	bar := manifestbuilder.New(f, "bar").
		WithK8sYAML(testyaml.SanchoYAML).
		WithImageTarget(NewSanchoDockerBuildImageTarget(f)).
		Build()
	a := f.run(bar)

	if assert.Error(t, a.Err) {
		assert.Equal(t, "Failed to connect to Docker: connection-error", a.Err.Error())
	}
}

func TestConfigsControllerDockerNotConnectedButNotRequired(t *testing.T) {
	f := newCCFixture(t)
	defer f.TearDown()

	assert.Equal(t, model.OrchestratorUnknown, f.docker.Orchestrator)
	f.addManifest("fe")
	f.docker.CheckConnectedErr = fmt.Errorf("connection-error")

	bar := manifestbuilder.New(f, "bar").
		WithK8sYAML(testyaml.SanchoYAML).
		Build()
	a := f.run(bar)

	assert.NoError(t, a.Err)
}

type ccFixture struct {
	*tempdir.TempDirFixture
	ctx        context.Context
	cancel     func()
	cc         *ConfigsController
	st         *store.Store
	getActions func() []store.Action
	tfl        *tiltfile.FakeTiltfileLoader
	fc         *testutils.FakeClock
	docker     *docker.FakeClient
	loopDone   chan struct{}
}

func newCCFixture(t *testing.T) *ccFixture {
	f := tempdir.NewTempDirFixture(t)
	st, getActions := store.NewStoreForTesting()
	tfl := tiltfile.NewFakeTiltfileLoader()
	d := docker.NewFakeClient()
	cc := NewConfigsController(tfl, d)
	fc := testutils.NewRandomFakeClock()
	cc.clock = fc.Clock()
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)
	loopDone := make(chan struct{})

	st.AddSubscriber(ctx, cc)
	go func() {
		err := st.Loop(ctx)
		testutils.FailOnNonCanceledErr(t, err, "store.Loop failed")
		close(loopDone)
	}()

	// configs_controller uses state.RelativeTiltfilePath, which is relative to wd
	// sometimes the original directory was invalid (e.g., it was another test's temp dir, which was deleted,
	// but not changed out of), and if it was already invalid, then let's not worry about it.
	f.Chdir()

	return &ccFixture{
		TempDirFixture: f,
		ctx:            ctx,
		cancel:         cancel,
		cc:             cc,
		st:             st,
		getActions:     getActions,
		tfl:            tfl,
		fc:             fc,
		docker:         d,
		loopDone:       loopDone,
	}
}

func (f *ccFixture) TearDown() {
	f.cancel()
	select {
	case <-f.loopDone:
	case <-time.After(2 * time.Second):
		f.T().Fatalf("Timeout waiting for store loop")
	}
	f.TempDirFixture.TearDown()
}

func (f *ccFixture) addManifest(name model.ManifestName) {
	state := f.st.LockMutableStateForTesting()
	m := manifestbuilder.New(f, name).WithK8sYAML(testyaml.SanchoYAML).Build()
	mt := store.NewManifestTarget(m)
	state.UpsertManifestTarget(mt)
	state.PendingConfigFileChanges["Tiltfile"] = time.Now()
	state.TiltfilePath = f.JoinPath("Tiltfile")
	f.st.UnlockMutableState()
}

func (f *ccFixture) run(m model.Manifest) ConfigsReloadedAction {
	f.st.SetUpSubscribersForTesting(f.ctx)

	f.tfl.Result = tiltfile.TiltfileLoadResult{
		Manifests: []model.Manifest{m},
	}

	f.st.NotifySubscribers(f.ctx)

	a := store.WaitForAction(f.T(), reflect.TypeOf(ConfigsReloadedAction{}), f.getActions)
	cra, ok := a.(ConfigsReloadedAction)
	if !ok {
		f.T().Fatalf("didn't get an action of type %T", ConfigsReloadedAction{})
	}

	return cra
}

const SanchoDockerfile = `
FROM go:1.10
ADD . .
RUN go install github.com/windmilleng/sancho
ENTRYPOINT /go/bin/sancho
`

var SanchoRef = container.MustParseSelector(testyaml.SanchoImage)

func NewSanchoDockerBuildImageTarget(f *ccFixture) model.ImageTarget {
	return model.MustNewImageTarget(SanchoRef).WithBuildDetails(model.DockerBuild{
		Dockerfile: SanchoDockerfile,
		BuildPath:  f.Path(),
	})
}
