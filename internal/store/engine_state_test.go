package store

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/model"
)

func TestStateToViewMultipleMounts(t *testing.T) {
	m := model.Manifest{
		Name: "foo",
		Mounts: []model.Mount{
			{
				LocalPath: "/a/b",
			},
			{
				LocalPath: "/a/b/c",
			},
		},
	}
	state := newState([]model.Manifest{m}, model.YAMLManifest{})
	ms := state.ManifestStates[m.Name]
	ms.CurrentlyBuildingFileChanges = []string{"/a/b/d", "/a/b/c/d/e"}
	ms.LastSuccessfulDeployEdits = []string{"/a/b/d", "/a/b/c/d/e"}
	ms.PendingFileChanges = map[string]bool{"/a/b/d": true, "/a/b/c/d/e": true}
	v := StateToView(*state)

	if !assert.Equal(t, 1, len(v.Resources)) {
		return
	}

	r := v.Resources[0]
	assert.Equal(t, []string{"d", "d/e"}, r.LastDeployEdits)

	sort.Strings(r.CurrentBuildEdits)
	assert.Equal(t, []string{"d", "d/e"}, r.CurrentBuildEdits)
	assert.Equal(t, []string{"d", "d/e"}, r.PendingBuildEdits)
}

func TestStateViewYAMLManifestNoYAML(t *testing.T) {
	m := model.NewYAMLManifest(model.ManifestName("GlobalYAML"), "", []string{})
	state := newState([]model.Manifest{}, m)
	v := StateToView(*state)

	assert.Equal(t, 0, len(v.Resources))
}

func TestStateViewYAMLManifestWithYAML(t *testing.T) {
	yaml := "yamlyaml"
	m := model.NewYAMLManifest(model.ManifestName("GlobalYAML"), yaml, []string{"global.yaml"})
	state := newState([]model.Manifest{}, m)
	v := StateToView(*state)

	assert.Equal(t, 1, len(v.Resources))

	r := v.Resources[0]
	assert.Equal(t, "", r.LastBuildError)
	assert.Equal(t, []string{"global.yaml"}, r.DirectoriesWatched)
}

func newState(manifests []model.Manifest, YAMLManifest model.YAMLManifest) *EngineState {
	ret := NewState()
	for _, m := range manifests {
		ret.ManifestStates[m.Name] = NewManifestState(m)
		ret.ManifestDefinitionOrder = append(ret.ManifestDefinitionOrder, m.Name)
	}
	ret.GlobalYAML = YAMLManifest
	ret.GlobalYAMLState = NewYAMLManifestState(YAMLManifest)

	return ret
}
