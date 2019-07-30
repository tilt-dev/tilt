package webview

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/k8s/testyaml"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

func TestStateToWebViewMultipleSyncs(t *testing.T) {
	m := model.Manifest{
		Name: "foo",
	}.WithImageTarget(model.ImageTarget{}.
		WithBuildDetails(model.FastBuild{
			Syncs: []model.Sync{
				{LocalPath: "/a/b"},
				{LocalPath: "/a/b/c"},
			},
		}),
	)

	state := newState([]model.Manifest{m})
	ms := state.ManifestTargets[m.Name].State
	ms.CurrentBuild.Edits = []string{"/a/b/d", "/a/b/c/d/e"}
	ms.BuildHistory = []model.BuildRecord{
		{Edits: []string{"/a/b/d", "/a/b/c/d/e"}},
	}
	ms.MutableBuildStatus(m.ImageTargets[0].ID()).PendingFileChanges =
		map[string]time.Time{"/a/b/d": time.Now(), "/a/b/c/d/e": time.Now()}
	v := StateToWebView(*state)

	if !assert.Equal(t, 2, len(v.Resources)) {
		return
	}

	r, _ := v.Resource(m.Name)
	assert.Equal(t, []string{"d", "d/e"}, r.LastBuild().Edits)

	sort.Strings(r.CurrentBuild.Edits)
	assert.Equal(t, []string{"d", "d/e"}, r.CurrentBuild.Edits)
	assert.Equal(t, []string{"d", "d/e"}, r.PendingBuildEdits)
}

func TestStateToWebViewPortForwards(t *testing.T) {
	m := model.Manifest{
		Name: "foo",
	}.WithDeployTarget(model.K8sTarget{
		PortForwards: []model.PortForward{
			{LocalPort: 8000, ContainerPort: 5000},
			{LocalPort: 7000, ContainerPort: 5001},
		},
	})
	state := newState([]model.Manifest{m})
	v := StateToWebView(*state)
	res, _ := v.Resource(m.Name)
	assert.Equal(t,
		[]string{"http://localhost:7000/", "http://localhost:8000/"},
		res.Endpoints)
}

func TestStateToViewUnresourcedYAMLManifest(t *testing.T) {
	m, err := k8s.NewK8sOnlyManifestFromYAML(testyaml.SanchoYAML)
	assert.NoError(t, err)
	state := newState([]model.Manifest{m})
	v := StateToWebView(*state)

	assert.Equal(t, 2, len(v.Resources))

	r, _ := v.Resource(m.Name)
	assert.Equal(t, nil, r.LastBuild().Error)

	expectedInfo := YAMLResourceInfo{K8sResources: []string{"sancho:deployment"}}
	assert.Equal(t, expectedInfo, r.ResourceInfo)
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

	m := model.Manifest{Name: "server"}
	targ := store.NewManifestTarget(m)
	targ.State = &store.ManifestState{}
	state.UpsertManifestTarget(targ)

	v := StateToWebView(*state)

	assert.False(t, v.NeedsAnalyticsNudge,
		"LastSuccessfulDeployTime not set, so NeedsNudge should not be set")

	targ.State = &store.ManifestState{LastSuccessfulDeployTime: time.Now()}
	state.UpsertManifestTarget(targ)

	v = StateToWebView(*state)
	assert.True(t, v.NeedsAnalyticsNudge)
}

func TestTriggerMode(t *testing.T) {
	state := newState(nil)
	m := model.Manifest{Name: "server"}
	targ := store.NewManifestTarget(m)
	targ.Manifest.TriggerMode = model.TriggerModeManual
	targ.State = &store.ManifestState{}
	state.UpsertManifestTarget(targ)

	v := StateToWebView(*state)
	assert.Equal(t, 2, len(v.Resources))

	newM, _ := v.Resource(model.ManifestName("server"))
	assert.Equal(t, model.TriggerModeManual, newM.TriggerMode)
}

func TestFeatureFlags(t *testing.T) {
	state := newState(nil)
	state.Features = map[string]bool{"foo_feature": true}

	v := StateToWebView(*state)
	assert.Equal(t, v.FeatureFlags, map[string]bool{"foo_feature": true})
}

func newState(manifests []model.Manifest) *store.EngineState {
	ret := store.NewState()
	for _, m := range manifests {
		ret.ManifestTargets[m.Name] = store.NewManifestTarget(m)
		ret.ManifestDefinitionOrder = append(ret.ManifestDefinitionOrder, m.Name)
	}

	return ret
}
