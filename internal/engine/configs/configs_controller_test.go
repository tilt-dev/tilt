package configs

import (
	"bytes"
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/container"
	ctrltiltfile "github.com/tilt-dev/tilt/internal/controllers/core/tiltfile"
	"github.com/tilt-dev/tilt/internal/controllers/fake"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/tiltfiles"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/internal/tiltfile"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
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
	f.onChange()
	f.popQueue()
	f.popQueue()

	require.NotNil(t, f.st.end)
	assert.Equal(t, []model.Manifest{bar}, f.st.end.Manifests)
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
	f.onChange()
	f.popQueue()
	f.popQueue()

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
	f.onChange()
	f.popQueue()
	f.popQueue()

	assert.NoError(t, f.st.end.Err)
}

func TestBuildReasonTrigger(t *testing.T) {
	f := newCCFixture(t)
	defer f.TearDown()

	f.addManifest("fe")
	bar := manifestbuilder.New(f, "bar").WithK8sYAML(testyaml.SanchoYAML).Build()

	state := f.st.LockMutableStateForTesting()
	state.AppendToTriggerQueue(model.MainTiltfileManifestName, model.BuildReasonFlagTriggerWeb)
	f.st.UnlockMutableState()

	f.setManifestResult(bar)
	f.onChange()
	f.popQueue()
	f.popQueue()

	assert.True(t, f.st.start.Reason.Has(model.BuildReasonFlagTriggerWeb),
		"expected build reason has flag: TriggerWeb")
}

func TestErrorLog(t *testing.T) {
	f := newCCFixture(t)
	defer f.TearDown()

	f.tfl.Result = tiltfile.TiltfileLoadResult{Error: fmt.Errorf("The goggles do nothing!")}
	f.onChange()
	f.popQueue()
	f.popQueue()

	assert.Contains(f.T(), f.st.out.String(), "ERROR LEVEL: The goggles do nothing!")
}

type testStore struct {
	*store.TestingStore
	out   *bytes.Buffer
	start *ctrltiltfile.ConfigsReloadStartedAction
	end   *ctrltiltfile.ConfigsReloadedAction
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

	start, ok := action.(ctrltiltfile.ConfigsReloadStartedAction)
	if ok {
		s.start = &start
	}

	end, ok := action.(ctrltiltfile.ConfigsReloadedAction)
	if ok {
		s.end = &end
	}

	tfa, ok := action.(tiltfiles.TiltfileUpsertAction)
	if ok {
		state := s.LockMutableStateForTesting()
		tiltfiles.HandleTiltfileUpsertAction(state, tfa)
		s.UnlockMutableState()
	}
}

type ccFixture struct {
	*tempdir.TempDirFixture
	ctx    context.Context
	cancel func()
	cc     *ConfigsController
	st     *testStore
	tfl    *tiltfile.FakeTiltfileLoader
	docker *docker.FakeClient
	tfr    *ctrltiltfile.Reconciler
	q      workqueue.RateLimitingInterface
}

func newCCFixture(t *testing.T) *ccFixture {
	f := tempdir.NewTempDirFixture(t)
	st := NewTestingStore()
	tfl := tiltfile.NewFakeTiltfileLoader()
	d := docker.NewFakeClient()
	tc := fake.NewFakeTiltClient()
	q := workqueue.NewRateLimitingQueue(
		workqueue.NewItemExponentialFailureRateLimiter(time.Millisecond, time.Millisecond))
	buildSource := ctrltiltfile.NewBuildSource()
	tfr := ctrltiltfile.NewReconciler(st, tfl, d, tc, v1alpha1.NewScheme(), buildSource)
	cc := NewConfigsController(tc, buildSource)
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()
	ctx, cancel := context.WithCancel(ctx)
	_ = buildSource.Start(ctx, handler.Funcs{}, q)

	// configs_controller uses state.RelativeTiltfilePath, which is relative to wd
	// sometimes the original directory was invalid (e.g., it was another test's temp dir, which was deleted,
	// but not changed out of), and if it was already invalid, then let's not worry about it.
	f.Chdir()

	state := st.LockMutableStateForTesting()
	state.DesiredTiltfilePath = f.JoinPath("Tiltfile")
	st.UnlockMutableState()

	// Simulate tiltfile initialization
	_ = cc.maybeCreateInitialTiltfile(ctx, st)
	_, _ = tfr.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: model.MainTiltfileManifestName.String()}})

	return &ccFixture{
		TempDirFixture: f,
		ctx:            ctx,
		cancel:         cancel,
		cc:             cc,
		st:             st,
		tfl:            tfl,
		docker:         d,
		tfr:            tfr,
		q:              q,
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

func (f *ccFixture) onChange() {
	_ = f.cc.OnChange(f.ctx, f.st, store.LegacyChangeSummary())
}

// Wait for the next item on the workqueue, then run reconcile on it.
func (f *ccFixture) popQueue() {
	f.T().Helper()

	done := make(chan error)
	go func() {
		item, _ := f.q.Get()
		_, err := f.tfr.Reconcile(f.ctx, item.(reconcile.Request))
		f.q.Done(item)
		done <- err
	}()

	select {
	case <-time.After(time.Second):
		f.T().Fatal("timeout waiting for workqueue")
	case err := <-done:
		assert.NoError(f.T(), err)
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
