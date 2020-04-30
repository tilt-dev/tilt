package manifestutils

import (
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"
)

func NewManifestTargetWithPod(m model.Manifest, pod store.Pod) *store.ManifestTarget {
	mt := store.NewManifestTarget(m)
	mt.State.RuntimeState = store.NewK8sRuntimeState(m.Name, pod)
	return mt
}
