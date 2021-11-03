package liveupdates

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/k8s/testyaml"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"
	"github.com/tilt-dev/tilt/internal/testutils/manifestbuilder"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

var SanchoRef = container.MustParseSelector(testyaml.SanchoImage)

func TestNeedsCrashRebuildLiveUpdateV1(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()

	iTarget := imageTarget()
	m := manifestbuilder.New(f, model.ManifestName("sancho")).
		WithK8sYAML(testyaml.SanchoYAML).
		WithLiveUpdateBAD().
		WithImageTarget(iTarget).
		Build()
	s := store.NewState()
	s.KubernetesResources["sancho"] = k8sResource()

	s.UpsertManifestTarget(store.NewManifestTarget(m))
	st, _ := s.ManifestState("sancho")
	st.LiveUpdatedContainerIDs = map[container.ID]bool{"crash": true}

	CheckForContainerCrash(s, "sancho")
	assert.True(t, st.NeedsRebuildFromCrash)
}

func TestNeedsCrashRebuildLiveUpdateV2(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()

	iTarget := imageTarget()
	iTarget.LiveUpdateReconciler = true
	m := manifestbuilder.New(f, model.ManifestName("sancho")).
		WithK8sYAML(testyaml.SanchoYAML).
		WithImageTarget(iTarget).
		Build()
	s := store.NewState()
	s.KubernetesResources["sancho"] = k8sResource()

	s.UpsertManifestTarget(store.NewManifestTarget(m))
	st, _ := s.ManifestState("sancho")
	st.LiveUpdatedContainerIDs = map[container.ID]bool{"crash": true}

	CheckForContainerCrash(s, "sancho")
	assert.False(t, st.NeedsRebuildFromCrash)
}

func imageTarget() model.ImageTarget {
	iTarget := model.MustNewImageTarget(SanchoRef)
	iTarget.LiveUpdateReconciler = true
	iTarget.LiveUpdateSpec = v1alpha1.LiveUpdateSpec{
		Selector: v1alpha1.LiveUpdateSelector{
			Kubernetes: &v1alpha1.LiveUpdateKubernetesSelector{
				Image: "sancho",
			},
		},
		Syncs: []v1alpha1.LiveUpdateSync{{LocalPath: "/", ContainerPath: "/"}},
	}
	return iTarget
}

func k8sResource() *k8sconv.KubernetesResource {
	return &k8sconv.KubernetesResource{
		FilteredPods: []v1alpha1.Pod{
			{
				Name:      "pod-1",
				Namespace: "default",
				Containers: []v1alpha1.Container{
					{
						Name:  "main",
						ID:    "main-id",
						Image: "sancho",
						State: v1alpha1.ContainerState{
							Running: &v1alpha1.ContainerStateRunning{},
						},
					},
				},
			},
		},
	}
}
