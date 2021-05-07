package webview

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/engine/configs"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/internal/timecmp"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	proto_webview "github.com/tilt-dev/tilt/pkg/webview"
)

var fooManifest = model.Manifest{Name: "foo"}.WithDeployTarget(model.K8sTarget{})

func stateToProtoView(t *testing.T, s store.EngineState) *proto_webview.View {
	s.UISessions[types.NamespacedName{Name: UISessionName}] = ToUISession(s)
	v, err := StateToProtoView(s, 0)
	if err != nil {
		t.Fatal(err)
	}

	return v
}

func TestStateToWebViewRelativeEditPaths(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()

	m := model.Manifest{
		Name: "foo",
	}.WithDeployTarget(model.K8sTarget{}).WithImageTarget(model.ImageTarget{}.
		WithBuildDetails(model.DockerBuild{BuildPath: f.JoinPath("a", "b", "c")}))

	state := newState([]model.Manifest{m})
	ms := state.ManifestTargets[m.Name].State
	ms.BuildHistory = []model.BuildRecord{
		{},
	}
	ms.MutableBuildStatus(m.ImageTargets[0].ID()).PendingFileChanges =
		map[string]time.Time{
			f.JoinPath("a", "b", "c", "foo"):    time.Now(),
			f.JoinPath("a", "b", "c", "d", "e"): time.Now(),
		}
	v := stateToProtoView(t, *state)

	require.Len(t, v.UiResources, 2)
}

func TestStateToWebViewPortForwards(t *testing.T) {
	m := model.Manifest{
		Name: "foo",
	}.WithDeployTarget(model.K8sTarget{
		PortForwards: []model.PortForward{
			{LocalPort: 8000, ContainerPort: 5000},
			{LocalPort: 7000, ContainerPort: 5001},
			{LocalPort: 5000, ContainerPort: 5002, Host: "127.0.0.2", Name: "dashboard"},
			{LocalPort: 6000, ContainerPort: 5003, Name: "debugger"},
		},
	})
	state := newState([]model.Manifest{m})
	v := stateToProtoView(t, *state)

	expected := []v1alpha1.UIResourceLink{
		v1alpha1.UIResourceLink{URL: "http://localhost:8000/"},
		v1alpha1.UIResourceLink{URL: "http://localhost:7000/"},
		v1alpha1.UIResourceLink{URL: "http://127.0.0.2:5000/", Name: "dashboard"},
		v1alpha1.UIResourceLink{URL: "http://localhost:6000/", Name: "debugger"},
	}
	res, _ := findResource(m.Name, v)
	assert.Equal(t, expected, res.EndpointLinks)
}

func TestStateToWebViewLinksAndPortForwards(t *testing.T) {
	m := model.Manifest{
		Name: "foo",
	}.WithDeployTarget(model.K8sTarget{
		PortForwards: []model.PortForward{
			{LocalPort: 8000, ContainerPort: 5000},
			{LocalPort: 8001, ContainerPort: 5001, Name: "debugger"},
		},
		Links: []model.Link{
			model.MustNewLink("www.apple.edu", "apple"),
			model.MustNewLink("www.zombo.com", "zombo"),
		},
	})
	state := newState([]model.Manifest{m})
	v := stateToProtoView(t, *state)

	expected := []v1alpha1.UIResourceLink{
		v1alpha1.UIResourceLink{URL: "www.apple.edu", Name: "apple"},
		v1alpha1.UIResourceLink{URL: "www.zombo.com", Name: "zombo"},
		v1alpha1.UIResourceLink{URL: "http://localhost:8000/"},
		v1alpha1.UIResourceLink{URL: "http://localhost:8001/", Name: "debugger"},
	}
	res, _ := findResource(m.Name, v)
	assert.Equal(t, expected, res.EndpointLinks)
}

func TestStateToWebViewLocalResourceLink(t *testing.T) {
	m := model.Manifest{
		Name: "foo",
	}.WithDeployTarget(model.LocalTarget{
		Links: []model.Link{
			model.MustNewLink("www.apple.edu", "apple"),
			model.MustNewLink("www.zombo.com", "zombo"),
		},
	})
	state := newState([]model.Manifest{m})
	v := stateToProtoView(t, *state)

	expected := []v1alpha1.UIResourceLink{
		v1alpha1.UIResourceLink{URL: "www.apple.edu", Name: "apple"},
		v1alpha1.UIResourceLink{URL: "www.zombo.com", Name: "zombo"},
	}
	res, _ := findResource(m.Name, v)
	assert.Equal(t, expected, res.EndpointLinks)
}

func TestStateToViewUnresourcedYAMLManifest(t *testing.T) {
	m, err := k8s.NewK8sOnlyManifestFromYAML(testyaml.SanchoYAML)
	assert.NoError(t, err)
	state := newState([]model.Manifest{m})
	v := stateToProtoView(t, *state)

	assert.Equal(t, 2, len(v.UiResources))

	r, _ := findResource(m.Name, v)
	assert.Equal(t, "", lastBuild(r).Error)
}

func TestStateToViewK8sTargetsIncludeDisplayNames(t *testing.T) {
	displayNames := []string{"foo:namespace", "foo:secret"}
	m := model.Manifest{Name: "foo"}.WithDeployTarget(model.K8sTarget{DisplayNames: displayNames})
	state := newState([]model.Manifest{m})
	v := stateToProtoView(t, *state)

	assert.Equal(t, 2, len(v.UiResources))

	r, _ := findResource(m.Name, v)

	assert.Equal(t, r.K8sResourceInfo.DisplayNames, displayNames)
}

func TestStateToViewTiltfileLog(t *testing.T) {
	es := newState([]model.Manifest{})
	spanID := configs.SpanIDForLoadCount(1)
	es.LogStore.Append(
		store.NewLogAction(store.TiltfileManifestName, spanID, logger.InfoLvl, nil, []byte("hello")),
		nil)
	v := stateToProtoView(t, *es)
	_, ok := findResource("(Tiltfile)", v)
	require.True(t, ok, "no resource named (Tiltfile) found")
	assert.Equal(t, "hello", string(v.LogList.Segments[0].Text))
	assert.Equal(t, "(Tiltfile)", string(v.LogList.Spans[string(spanID)].ManifestName))
}

func TestRelativeTiltfilePath(t *testing.T) {
	es := newState([]model.Manifest{})
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	es.TiltfilePath = filepath.Join(wd, "Tiltfile")

	actual, err := es.RelativeTiltfilePath()
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "Tiltfile", actual)
}

func TestNeedsNudgeSet(t *testing.T) {
	state := newState(nil)

	m := fooManifest
	targ := store.NewManifestTarget(m)
	targ.State = &store.ManifestState{}
	state.UpsertManifestTarget(targ)

	v := stateToProtoView(t, *state)

	assert.False(t, v.UiSession.Status.NeedsAnalyticsNudge,
		"LastSuccessfulDeployTime not set, so NeedsNudge should not be set")

	targ.State = &store.ManifestState{LastSuccessfulDeployTime: time.Now()}
	state.UpsertManifestTarget(targ)

	v = stateToProtoView(t, *state)
	assert.True(t, v.UiSession.Status.NeedsAnalyticsNudge)
}

func TestTriggerMode(t *testing.T) {
	state := newState(nil)
	m := fooManifest
	targ := store.NewManifestTarget(m)
	targ.Manifest.TriggerMode = model.TriggerModeManualWithAutoInit
	targ.State = &store.ManifestState{}
	state.UpsertManifestTarget(targ)

	v := stateToProtoView(t, *state)
	assert.Equal(t, 2, len(v.UiResources))

	newM, _ := findResource(model.ManifestName("foo"), v)
	assert.Equal(t, model.TriggerModeManualWithAutoInit, model.TriggerMode(newM.TriggerMode))
}

func TestFeatureFlags(t *testing.T) {
	state := newState(nil)
	state.Features = map[string]bool{"foo_feature": true}

	v := stateToProtoView(t, *state)
	assert.Equal(t, v.UiSession.Status.FeatureFlags, []v1alpha1.UIFeatureFlag{
		v1alpha1.UIFeatureFlag{Name: "foo_feature", Value: true},
	})
}

func TestReadinessCheckFailing(t *testing.T) {
	m := model.Manifest{
		Name: "foo",
	}.WithDeployTarget(model.K8sTarget{})
	state := newState([]model.Manifest{m})
	state.ManifestTargets[m.Name].State.RuntimeState = store.K8sRuntimeState{
		Pods: map[k8s.PodID]*v1alpha1.Pod{
			"pod id": {
				Status: "Running",
				Phase:  "Running",
				Containers: []v1alpha1.Container{
					{
						Ready: false,
					},
				},
			},
		},
	}

	v := stateToProtoView(t, *state)
	rv, ok := findResource(m.Name, v)
	require.True(t, ok)
	require.Equal(t, v1alpha1.RuntimeStatusPending, v1alpha1.RuntimeStatus(rv.RuntimeStatus))
}

func TestLocalResource(t *testing.T) {
	cmd := model.Cmd{
		Argv: []string{"make", "test"},
		Dir:  "path/to/tiltfile",
	}
	lt := model.NewLocalTarget("my-local", cmd, model.Cmd{}, []string{"/foo/bar", "/baz/qux"})
	m := model.Manifest{
		Name: "test",
	}.WithDeployTarget(lt)

	state := newState([]model.Manifest{m})
	lrs := store.LocalRuntimeState{Status: v1alpha1.RuntimeStatusNotApplicable}
	state.ManifestTargets[m.Name].State.RuntimeState = lrs
	v := stateToProtoView(t, *state)

	assert.Equal(t, 2, len(v.UiResources))
	r := v.UiResources[1]
	rs := r.Status
	assert.Equal(t, "test", r.Name)
	assert.Equal(t, v1alpha1.RuntimeStatusNotApplicable, rs.RuntimeStatus)
	require.Len(t, rs.Specs, 1)
	spec := rs.Specs[0]
	require.Equal(t, v1alpha1.UIResourceTargetTypeLocal, spec.Type)
	require.False(t, spec.HasLiveUpdate)
}

func TestBuildHistory(t *testing.T) {
	br1 := model.BuildRecord{
		StartTime:  time.Now().Add(-1 * time.Hour),
		FinishTime: time.Now().Add(-50 * time.Minute),
		Reason:     model.BuildReasonFlagInit,
		BuildTypes: []model.BuildType{model.BuildTypeImage, model.BuildTypeK8s},
	}
	br2 := model.BuildRecord{
		StartTime:  time.Now().Add(-45 * time.Minute),
		FinishTime: time.Now().Add(-44 * time.Minute),
		Reason:     model.BuildReasonFlagChangedFiles,
		BuildTypes: []model.BuildType{model.BuildTypeLiveUpdate},
	}
	br3 := model.BuildRecord{
		StartTime:  time.Now().Add(-20 * time.Minute),
		FinishTime: time.Now().Add(-19 * time.Minute),
		Reason:     model.BuildReasonFlagCrash,
		BuildTypes: []model.BuildType{model.BuildTypeImage, model.BuildTypeK8s},
	}
	buildRecords := []model.BuildRecord{br1, br2, br3}

	m := model.Manifest{Name: "foo"}.WithDeployTarget(model.K8sTarget{})
	state := newState([]model.Manifest{m})
	state.ManifestTargets[m.Name].State.BuildHistory = buildRecords

	v := stateToProtoView(t, *state)
	require.Equal(t, 2, len(v.UiResources))
	r := v.UiResources[1]
	require.Equal(t, "foo", r.Name)

	rs := r.Status
	require.Len(t, rs.BuildHistory, 3)

	for i, actual := range rs.BuildHistory {
		expected := buildRecords[i]
		timecmp.AssertTimeEqual(t, expected.StartTime, actual.StartTime)
		timecmp.AssertTimeEqual(t, expected.FinishTime, actual.FinishTime)
		require.Equal(t, i == 2, actual.IsCrashRebuild)
	}
}

func TestSpecs(t *testing.T) {
	lu, err := model.NewLiveUpdate(
		[]model.LiveUpdateStep{model.LiveUpdateSyncStep{Source: "foo", Dest: "bar"}}, ".")
	require.NoError(t, err)
	luTarg := model.ImageTarget{}.WithBuildDetails(model.DockerBuild{LiveUpdate: lu})

	mNoLiveUpd := model.Manifest{Name: "noLiveUpd"}.WithImageTarget(model.ImageTarget{}).WithDeployTarget(model.K8sTarget{})
	mLiveUpd := model.Manifest{Name: "liveUpd"}.WithImageTarget(luTarg).WithDeployTarget(model.K8sTarget{})
	mLocal := model.Manifest{Name: "local"}.WithDeployTarget(model.LocalTarget{})

	expected := []struct {
		name          string
		targetTypes   []v1alpha1.UIResourceTargetType
		hasLiveUpdate bool
	}{
		{"noLiveUpd", []v1alpha1.UIResourceTargetType{v1alpha1.UIResourceTargetTypeImage, v1alpha1.UIResourceTargetTypeKubernetes}, false},
		{"liveUpd", []v1alpha1.UIResourceTargetType{v1alpha1.UIResourceTargetTypeImage, v1alpha1.UIResourceTargetTypeKubernetes}, true},
		{"local", []v1alpha1.UIResourceTargetType{v1alpha1.UIResourceTargetTypeLocal}, false},
	}
	state := newState([]model.Manifest{mNoLiveUpd, mLiveUpd, mLocal})
	v := stateToProtoView(t, *state)

	require.Equal(t, 4, len(v.UiResources))
	for i, r := range v.UiResources {
		if i == 0 {
			continue // skip Tiltfile
		}
		expected := expected[i-1]
		require.Equal(t, expected.name, r.Name, "name mismatch for resource at index %d", i)
		observedTypes := []v1alpha1.UIResourceTargetType{}
		var iTargHasLU bool
		rs := r.Status
		for _, spec := range rs.Specs {
			observedTypes = append(observedTypes, spec.Type)
			if spec.Type == v1alpha1.UIResourceTargetTypeImage {
				iTargHasLU = spec.HasLiveUpdate
			}
		}
		require.ElementsMatch(t, expected.targetTypes, observedTypes, "for resource %q", r.Name)
		require.Equal(t, expected.hasLiveUpdate, iTargHasLU, "for resource %q", r.Name)
	}
}

func findResource(n model.ManifestName, view *proto_webview.View) (v1alpha1.UIResourceStatus, bool) {
	for _, r := range view.UiResources {
		if r.Name == n.String() {
			return r.Status, true
		}
	}
	return v1alpha1.UIResourceStatus{}, false
}

func lastBuild(r v1alpha1.UIResourceStatus) v1alpha1.UIBuildTerminated {
	if len(r.BuildHistory) == 0 {
		return v1alpha1.UIBuildTerminated{}
	}

	return r.BuildHistory[0]
}
