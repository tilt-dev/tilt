package webview

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/k8s"
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
	state := newState([]model.Manifest{m}, model.Manifest{})
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
	state := newState([]model.Manifest{m}, model.Manifest{})
	v := StateToWebView(*state)
	res, _ := v.Resource(m.Name)
	assert.Equal(t,
		[]string{"http://localhost:7000/", "http://localhost:8000/"},
		res.Endpoints)
}

func TestStateViewYAMLManifestNoYAML(t *testing.T) {
	m := k8s.NewK8sOnlyManifestForTesting("GlobalYAML", "")
	state := newState([]model.Manifest{}, m)
	v := StateToWebView(*state)

	assert.Equal(t, 1, len(v.Resources))
}

func TestStateViewYAMLManifestWithYAML(t *testing.T) {
	m := k8s.NewK8sOnlyManifestForTesting("GlobalYAML", "yamlyaml")
	state := newState([]model.Manifest{}, m)
	state.ConfigFiles = []string{"global.yaml"}
	v := StateToWebView(*state)

	assert.Equal(t, 2, len(v.Resources))

	r, _ := v.Resource(m.Name)
	assert.Equal(t, nil, r.LastBuild().Error)
	assert.Equal(t, []string{"global.yaml"}, r.DirectoriesWatched)
}

func TestRelativeTiltfilePath(t *testing.T) {
	es := newState([]model.Manifest{}, model.Manifest{})
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

func newState(manifests []model.Manifest, YAMLManifest model.Manifest) *store.EngineState {
	ret := store.NewState()
	for _, m := range manifests {
		ret.ManifestTargets[m.Name] = store.NewManifestTarget(m)
		ret.ManifestDefinitionOrder = append(ret.ManifestDefinitionOrder, m.Name)
	}
	ret.GlobalYAML = YAMLManifest
	ret.GlobalYAMLState = store.NewYAMLManifestState()

	return ret
}
