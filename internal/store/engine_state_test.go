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
	state := newState([]model.Manifest{m})
	ms := state.ManifestStates[m.Name]
	ms.NewFileChangesInCurrentBuild = []string{"/a/b/d", "/a/b/c/d/e"}
	ms.LastSuccessfulDeployEdits = []string{"/a/b/d", "/a/b/c/d/e"}
	ms.FileChangesSinceLastBuild = map[string]bool{"/a/b/d": true, "/a/b/c/d/e": true}
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

func newState(manifests []model.Manifest) *EngineState {
	ret := NewState()
	for _, m := range manifests {
		ret.ManifestStates[m.Name] = NewManifestState(m)
		ret.ManifestDefinitionOrder = append(ret.ManifestDefinitionOrder, m.Name)
	}

	return ret
}
