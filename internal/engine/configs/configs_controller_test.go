package configs

import (
	"context"
	"fmt"
	"reflect"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/internal/tiltfile"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestConfigsController(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TODO(nick): investigate")
	}
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
	ctx    context.Context
	cancel func()
	cc     *ConfigsController
	st     *store.TestingStore
	tfl    *tiltfile.FakeTiltfileLoader
	fc     *testutils.FakeClock
	docker *docker.FakeClient
}

func newCCFixture(t *testing.T) *ccFixture {
	f := tempdir.NewTempDirFixture(t)
	st := store.NewTestingStore()
	tfl := tiltfile.NewFakeTiltfileLoader()
	d := docker.NewFakeClient()
	cc := NewConfigsController(tfl, d)
	fc := testutils.NewRandomFakeClock()
	cc.clock = fc.Clock()
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)

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
		tfl:            tfl,
		fc:             fc,
		docker:         d,
	}
}

func (f *ccFixture) TearDown() {
	f.cancel()
	f.TempDirFixture.TearDown()
	f.st.AssertNoErrorActions(f.T())
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
	f.tfl.Result = tiltfile.TiltfileLoadResult{
		Manifests: []model.Manifest{m},
	}

	f.cc.OnChange(f.ctx, f.st)

	a := store.WaitForAction(f.T(), reflect.TypeOf(ConfigsReloadedAction{}), f.st.Actions)
	cra, ok := a.(ConfigsReloadedAction)
	if !ok {
		f.T().Fatalf("didn't get an action of type %T", ConfigsReloadedAction{})
	}

	return cra
}

const SanchoDockerfile = `
FROM go:1.10
ADD . .
RUN go install github.com/tilt-dev/sancho
ENTRYPOINT /go/bin/sancho
`

var SanchoRef = container.MustParseSelector(testyaml.SanchoImage)

func NewSanchoDockerBuildImageTarget(f *ccFixture) model.ImageTarget {
	return model.MustNewImageTarget(SanchoRef).WithBuildDetails(model.DockerBuild{
		Dockerfile: SanchoDockerfile,
		BuildPath:  f.Path(),
	})
}
