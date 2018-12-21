package store

import (
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/model"
)

func TestStateToViewMultipleMounts(t *testing.T) {
	m := model.Manifest{
		Name: "foo",
		DockerInfo: model.DockerInfo{}.
			WithBuildDetails(model.FastBuild{
				Mounts: []model.Mount{
					{LocalPath: "/a/b"},
					{LocalPath: "/a/b/c"},
				},
			}),
	}
	state := newState([]model.Manifest{m}, model.YAMLManifest{})
	ms := state.ManifestStates[m.Name]
	ms.CurrentBuild.Edits = []string{"/a/b/d", "/a/b/c/d/e"}
	ms.BuildHistory = []model.BuildStatus{
		{Edits: []string{"/a/b/d", "/a/b/c/d/e"}},
	}
	ms.PendingFileChanges = map[string]time.Time{"/a/b/d": time.Now(), "/a/b/c/d/e": time.Now()}
	v := StateToView(*state)

	if !assert.Equal(t, 1, len(v.Resources)) {
		return
	}

	r := v.Resources[0]
	assert.Equal(t, []string{"d", "d/e"}, r.LastBuild().Edits)

	sort.Strings(r.CurrentBuild.Edits)
	assert.Equal(t, []string{"d", "d/e"}, r.CurrentBuild.Edits)
	assert.Equal(t, []string{"d", "d/e"}, r.PendingBuildEdits)
}

func TestStateToViewPortForwards(t *testing.T) {
	m := model.Manifest{
		Name: "foo",
	}.WithDeployInfo(model.K8sInfo{
		PortForwards: []model.PortForward{
			{LocalPort: 8000, ContainerPort: 5000},
			{LocalPort: 7000, ContainerPort: 5001},
		},
	})
	state := newState([]model.Manifest{m}, model.YAMLManifest{})
	v := StateToView(*state)
	assert.Equal(t,
		[]string{"http://localhost:7000/", "http://localhost:8000/"},
		v.Resources[0].Endpoints)
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
	assert.Equal(t, nil, r.LastBuild().Error)
	assert.Equal(t, []string{"global.yaml"}, r.DirectoriesWatched)
}

func TestMostRecentPod(t *testing.T) {
	podA := Pod{PodID: "pod-a", StartedAt: time.Now()}
	podB := Pod{PodID: "pod-b", StartedAt: time.Now().Add(time.Minute)}
	podC := Pod{PodID: "pod-c", StartedAt: time.Now().Add(-time.Minute)}
	podSet := NewPodSet(podA, podB, podC)
	assert.Equal(t, "pod-b", podSet.MostRecentPod().PodID.String())
}

func TestEmptyState(t *testing.T) {
	es := newState([]model.Manifest{}, model.YAMLManifest{})

	v := StateToView(*es)
	assert.Equal(t, emptyTiltfileMsg, v.TiltfileErrorMessage)

	yaml := "yamlyaml"
	m := model.NewYAMLManifest(model.ManifestName("GlobalYAML"), yaml, []string{"global.yaml"})
	nes := newState([]model.Manifest{}, m)
	v = StateToView(*nes)
	assert.Equal(t, "", v.TiltfileErrorMessage)

	m2 := model.Manifest{
		Name: "foo",
		DockerInfo: model.DockerInfo{}.
			WithBuildDetails(model.FastBuild{
				Mounts: []model.Mount{
					{LocalPath: "/a/b"},
					{LocalPath: "/a/b/c"},
				},
			}),
	}

	nes = newState([]model.Manifest{m2}, model.YAMLManifest{})
	v = StateToView(*nes)
	assert.Equal(t, "", v.TiltfileErrorMessage)
}

func newState(manifests []model.Manifest, YAMLManifest model.YAMLManifest) *EngineState {
	ret := NewState()
	for _, m := range manifests {
		ret.ManifestStates[m.Name] = NewManifestState(m)
		ret.ManifestDefinitionOrder = append(ret.ManifestDefinitionOrder, m.Name)
	}
	ret.GlobalYAML = YAMLManifest
	ret.GlobalYAMLState = NewYAMLManifestState()

	return ret
}
