package manifestutils

import (
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
)

func NewManifestTargetWithPod(m model.Manifest, pod store.Pod) *store.ManifestTarget {
	mt := store.NewManifestTarget(m)
	mt.State.RuntimeState = store.NewK8sRuntimeStateWithPods(m, pod)
	return mt
}
