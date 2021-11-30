package webview

import (
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ctrltiltfile "github.com/tilt-dev/tilt/internal/controllers/core/tiltfile"
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

func completeProtoView(t *testing.T, s store.EngineState) *proto_webview.View {
	st := store.NewTestingStore()
	st.SetState(s)

	view, err := LogUpdate(st, 0)
	require.NoError(t, err)

	view.UiSession = ToUISession(s)

	resources, err := ToUIResourceList(s, make(map[string][]v1alpha1.DisableSource))
	require.NoError(t, err)
	view.UiResources = resources

	sortUIResources(view.UiResources, s.ManifestDefinitionOrder)

	return view
}

func TestStateToWebViewRelativeEditPaths(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()

	m := model.Manifest{
		Name: "foo",
	}.WithDeployTarget(model.K8sTarget{}).WithImageTarget(model.ImageTarget{}.
		WithDockerImage(v1alpha1.DockerImageSpec{Context: f.JoinPath("a", "b", "c")}))

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
	v := completeProtoView(t, *state)

	require.Len(t, v.UiResources, 2)
}

func TestStateToWebViewPortForwards(t *testing.T) {
	m := model.Manifest{
		Name: "foo",
	}.WithDeployTarget(model.K8sTarget{
		KubernetesApplySpec: v1alpha1.KubernetesApplySpec{
			PortForwardTemplateSpec: &v1alpha1.PortForwardTemplateSpec{
				Forwards: []v1alpha1.Forward{
					{LocalPort: 8000, ContainerPort: 5000},
					{LocalPort: 7000, ContainerPort: 5001},
					{LocalPort: 5000, ContainerPort: 5002, Host: "127.0.0.2", Name: "dashboard"},
					{LocalPort: 6000, ContainerPort: 5003, Name: "debugger"},
				},
			},
		},
	})
	state := newState([]model.Manifest{m})
	v := completeProtoView(t, *state)

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
		KubernetesApplySpec: v1alpha1.KubernetesApplySpec{
			PortForwardTemplateSpec: &v1alpha1.PortForwardTemplateSpec{
				Forwards: []v1alpha1.Forward{
					{LocalPort: 8000, ContainerPort: 5000},
					{LocalPort: 8001, ContainerPort: 5001, Name: "debugger"},
				},
			},
		},
		Links: []model.Link{
			model.MustNewLink("www.apple.edu", "apple"),
			model.MustNewLink("www.zombo.com", "zombo"),
		},
	})
	state := newState([]model.Manifest{m})
	v := completeProtoView(t, *state)

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
	v := completeProtoView(t, *state)

	expected := []v1alpha1.UIResourceLink{
		v1alpha1.UIResourceLink{URL: "www.apple.edu", Name: "apple"},
		v1alpha1.UIResourceLink{URL: "www.zombo.com", Name: "zombo"},
	}
	res, _ := findResource(m.Name, v)
	assert.Equal(t, expected, res.EndpointLinks)
}

func TestStateToViewUnresourcedYAMLManifest(t *testing.T) {
	mn := model.UnresourcedYAMLManifestName
	m := model.Manifest{Name: mn}.WithDeployTarget(k8s.MustTarget(mn.TargetName(), testyaml.SanchoYAML))
	state := newState([]model.Manifest{m})
	v := completeProtoView(t, *state)

	assert.Equal(t, 2, len(v.UiResources))

	r, _ := findResource(m.Name, v)
	assert.Equal(t, "", lastBuild(r).Error)
}

func TestStateToViewK8sTargetsIncludeDisplayNames(t *testing.T) {
	m := model.Manifest{Name: "foo"}.WithDeployTarget(model.K8sTarget{})
	state := newState([]model.Manifest{m})
	krs := state.ManifestTargets["foo"].State.K8sRuntimeState()
	krs.ApplyFilter = &k8sconv.KubernetesApplyFilter{
		DeployedRefs: []v1.ObjectReference{
			{Kind: "Namespace", Name: "foo"},
			{Kind: "Secret", Name: "foo"},
		},
	}
	state.ManifestTargets["foo"].State.RuntimeState = krs

	v := completeProtoView(t, *state)

	assert.Equal(t, 2, len(v.UiResources))

	r, _ := findResource(m.Name, v)

	assert.Equal(t, []string{"foo:namespace", "foo:secret"}, r.K8sResourceInfo.DisplayNames)
}

func TestStateToViewTiltfileLog(t *testing.T) {
	es := newState([]model.Manifest{})
	spanID := ctrltiltfile.SpanIDForLoadCount("(Tiltfile)", 1)
	es.LogStore.Append(
		store.NewLogAction(store.MainTiltfileManifestName, spanID, logger.InfoLvl, nil, []byte("hello")),
		nil)
	v := completeProtoView(t, *es)
	_, ok := findResource("(Tiltfile)", v)
	require.True(t, ok, "no resource named (Tiltfile) found")
	assert.Equal(t, "hello", string(v.LogList.Segments[0].Text))
	assert.Equal(t, "(Tiltfile)", string(v.LogList.Spans[string(spanID)].ManifestName))
}

func TestNeedsNudgeSet(t *testing.T) {
	state := newState(nil)

	m := fooManifest
	targ := store.NewManifestTarget(m)
	targ.State = &store.ManifestState{}
	state.UpsertManifestTarget(targ)

	v := completeProtoView(t, *state)

	assert.False(t, v.UiSession.Status.NeedsAnalyticsNudge,
		"LastSuccessfulDeployTime not set, so NeedsNudge should not be set")

	targ.State = &store.ManifestState{LastSuccessfulDeployTime: time.Now()}
	state.UpsertManifestTarget(targ)

	v = completeProtoView(t, *state)
	assert.True(t, v.UiSession.Status.NeedsAnalyticsNudge)
}

func TestTriggerMode(t *testing.T) {
	state := newState(nil)
	m := fooManifest
	targ := store.NewManifestTarget(m)
	targ.Manifest.TriggerMode = model.TriggerModeManualWithAutoInit
	targ.State = &store.ManifestState{}
	state.UpsertManifestTarget(targ)

	v := completeProtoView(t, *state)
	assert.Equal(t, 2, len(v.UiResources))

	newM, _ := findResource(model.ManifestName("foo"), v)
	assert.Equal(t, model.TriggerModeManualWithAutoInit, model.TriggerMode(newM.TriggerMode))
}

func TestFeatureFlags(t *testing.T) {
	state := newState(nil)
	state.Features = map[string]bool{"foo_feature": true}

	v := completeProtoView(t, *state)
	assert.Equal(t, v.UiSession.Status.FeatureFlags, []v1alpha1.UIFeatureFlag{
		v1alpha1.UIFeatureFlag{Name: "foo_feature", Value: true},
	})
}

func TestReadinessCheckFailing(t *testing.T) {
	m := model.Manifest{
		Name: "foo",
	}.WithDeployTarget(model.K8sTarget{})
	state := newState([]model.Manifest{m})
	state.ManifestTargets[m.Name].State.RuntimeState = store.NewK8sRuntimeStateWithPods(m, v1alpha1.Pod{
		Name:   "pod-id",
		Status: "Running",
		Phase:  "Running",
		Containers: []v1alpha1.Container{
			{
				Ready: false,
			},
		},
	})

	v := completeProtoView(t, *state)
	rv, ok := findResource(m.Name, v)
	require.True(t, ok)
	require.Equal(t, v1alpha1.RuntimeStatusPending, v1alpha1.RuntimeStatus(rv.RuntimeStatus))
	require.Equal(t, "False", string(readyCondition(rv).Status))
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
	v := completeProtoView(t, *state)

	assert.Equal(t, 2, len(v.UiResources))
	r := v.UiResources[1]
	rs := r.Status
	assert.Equal(t, "test", r.Name)
	assert.Equal(t, v1alpha1.RuntimeStatusNotApplicable, rs.RuntimeStatus)
	require.Equal(t, "False", string(readyCondition(rs).Status))
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

	v := completeProtoView(t, *state)
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
	luSpec := v1alpha1.LiveUpdateSpec{
		BasePath: ".",
		Syncs:    []v1alpha1.LiveUpdateSync{{LocalPath: "foo", ContainerPath: "bar"}},
	}
	luTarg := model.ImageTarget{}.WithLiveUpdateSpec("sancho", luSpec).WithBuildDetails(model.DockerBuild{})

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
	v := completeProtoView(t, *state)

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

func TestDisableResourceStatus(t *testing.T) {
	m1 := model.Manifest{Name: "m1"}.WithDeployTarget(model.LocalTarget{})
	m2 := model.Manifest{Name: "m2"}.WithDeployTarget(model.LocalTarget{})
	m3 := model.Manifest{Name: "m3"}.WithDeployTarget(model.LocalTarget{})
	state := newState([]model.Manifest{m1, m2, m3})

	state.ConfigMaps = map[string]*v1alpha1.ConfigMap{
		"disable-m1":  {Data: map[string]string{"isDisabled": "true"}},
		"disable-m2a": {Data: map[string]string{"isDisabled": "true"}},
		"disable-m2b": {Data: map[string]string{"isDisabled": "true"}},
		"disable-m2c": {Data: map[string]string{"isDisabled": "false"}},
	}

	disableSources := map[string][]v1alpha1.DisableSource{
		"m1": {{ConfigMap: &v1alpha1.ConfigMapDisableSource{Name: "disable-m1", Key: "isDisabled"}}},
		"m2": {
			{ConfigMap: &v1alpha1.ConfigMapDisableSource{Name: "disable-m2a", Key: "isDisabled"}},
			{ConfigMap: &v1alpha1.ConfigMapDisableSource{Name: "disable-m2b", Key: "isDisabled"}},
			{ConfigMap: &v1alpha1.ConfigMapDisableSource{Name: "disable-m2c", Key: "isDisabled"}},
		},
	}

	uiResources, err := ToUIResourceList(*state, disableSources)
	require.NoError(t, err)

	expected := []v1alpha1.DisableResourceStatus{
		{}, // The first UIResource is the Tiltfile
		{
			EnabledCount:  0,
			DisabledCount: 1,
			Sources:       disableSources["m1"],
		},
		{
			EnabledCount:  1,
			DisabledCount: 2,
			Sources:       disableSources["m2"],
		},
		{
			EnabledCount:  0,
			DisabledCount: 0,
			Sources:       nil,
		},
	}

	for i, uir := range uiResources {
		require.Equal(t, expected[i], uir.Status.DisableStatus)
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

func readyCondition(rs v1alpha1.UIResourceStatus) *v1alpha1.UIResourceCondition {
	for _, c := range rs.Conditions {
		if c.Type == v1alpha1.UIResourceReady {
			return &c
		}
	}
	return nil
}
