package configs

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/internal/tiltfile"
	"github.com/tilt-dev/tilt/pkg/logger"
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
	f.setManifestResult(bar)
	_ = f.cc.OnChange(f.ctx, f.st, store.LegacyChangeSummary())

	expected := &ConfigsReloadedAction{
		Manifests:  []model.Manifest{bar},
		FinishTime: f.fc.Times[1],
	}

	assert.Equal(t, expected, f.st.end)
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
	f.setManifestResult(bar)
	_ = f.cc.OnChange(f.ctx, f.st, store.LegacyChangeSummary())

	if assert.Error(t, f.st.end.Err) {
		assert.Equal(t, "Failed to connect to Docker: connection-error", f.st.end.Err.Error())
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
	f.setManifestResult(bar)
	_ = f.cc.OnChange(f.ctx, f.st, store.LegacyChangeSummary())

	assert.NoError(t, f.st.end.Err)
}

func TestBuildReasonTrigger(t *testing.T) {
	f := newCCFixture(t)
	defer f.TearDown()

	f.addManifest("fe")
	bar := manifestbuilder.New(f, "bar").WithK8sYAML(testyaml.SanchoYAML).Build()

	state := f.st.LockMutableStateForTesting()
	state.AppendToTriggerQueue(model.TiltfileManifestName, model.BuildReasonFlagTriggerWeb)
	f.st.UnlockMutableState()

	f.setManifestResult(bar)
	_ = f.cc.OnChange(f.ctx, f.st, store.LegacyChangeSummary())

	assert.True(t, f.st.start.Reason.Has(model.BuildReasonFlagTriggerWeb),
		"expected build reason has flag: TriggerWeb")
}

func TestErrorLog(t *testing.T) {
	f := newCCFixture(t)
	defer f.TearDown()

	f.tfl.Result = tiltfile.TiltfileLoadResult{Error: fmt.Errorf("The goggles do nothing!")}
	_ = f.cc.OnChange(f.ctx, f.st, store.LegacyChangeSummary())

	assert.Contains(f.T(), f.st.out.String(), "ERROR LEVEL: The goggles do nothing!")
}

type testStore struct {
	*store.TestingStore
	out   *bytes.Buffer
	start *ConfigsReloadStartedAction
	end   *ConfigsReloadedAction
}

func NewTestingStore() *testStore {
	return &testStore{
		TestingStore: store.NewTestingStore(),
		out:          bytes.NewBuffer(nil),
	}
}

func (s *testStore) Dispatch(action store.Action) {
	s.TestingStore.Dispatch(action)

	logAction, ok := action.(store.LogAction)
	if ok {
		level := ""
		if logAction.Level() == logger.ErrorLvl {
			level = "ERROR LEVEL:"
		}
		_, _ = fmt.Fprintf(s.out, "%s %s", level, logAction.Message())
	}

	start, ok := action.(ConfigsReloadStartedAction)
	if ok {
		s.start = &start
	}

	end, ok := action.(ConfigsReloadedAction)
	if ok {
		s.end = &end
	}
}

type ccFixture struct {
	*tempdir.TempDirFixture
	ctx    context.Context
	cancel func()
	cc     *ConfigsController
	st     *testStore
	tfl    *tiltfile.FakeTiltfileLoader
	fc     *testutils.FakeClock
	docker *docker.FakeClient
}

func newCCFixture(t *testing.T) *ccFixture {
	f := tempdir.NewTempDirFixture(t)
	st := NewTestingStore()
	tfl := tiltfile.NewFakeTiltfileLoader()
	d := docker.NewFakeClient()
	tc := fake.NewTiltClient()
	cc := NewConfigsController(tfl, d, tc)
	fc := testutils.NewRandomFakeClock()
	cc.clock = fc.Clock()
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)

	// configs_controller uses state.RelativeTiltfilePath, which is relative to wd
	// sometimes the original directory was invalid (e.g., it was another test's temp dir, which was deleted,
	// but not changed out of), and if it was already invalid, then let's not worry about it.
	f.Chdir()

	state := st.LockMutableStateForTesting()
	state.TiltfilePath = f.JoinPath("Tiltfile")
	state.TiltfileState.AddPendingFileChange(model.TargetID{
		Type: model.TargetTypeConfigs,
		Name: "singleton",
	}, f.JoinPath("Tiltfile"), time.Now())
	st.UnlockMutableState()

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
	f.st.UnlockMutableState()
}

func (f *ccFixture) setManifestResult(m model.Manifest) {
	f.tfl.Result = tiltfile.TiltfileLoadResult{
		Manifests: []model.Manifest{m},
	}
}

const SanchoDockerfile = `
FROM go:1.10
ADD . .
RUN go install github.com/tilt-dev/sancho
ENTRYPOINT /go/bin/sancho
`

var SanchoRef = container.MustParseSelector(testyaml.SanchoImage)

type pathFixture interface {
	Path() string
}

func NewSanchoDockerBuildImageTarget(f pathFixture) model.ImageTarget {
	return model.MustNewImageTarget(SanchoRef).WithBuildDetails(model.DockerBuild{
		Dockerfile: SanchoDockerfile,
		BuildPath:  f.Path(),
	})
}
