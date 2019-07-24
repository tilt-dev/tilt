package manifestutils

import (
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

func NewManifestTargetWithPod(m model.Manifest, pod store.Pod) *store.ManifestTarget {
	mt := store.NewManifestTarget(m)
	mt.State.PodSet = store.NewPodSet(pod)
	return mt
}
