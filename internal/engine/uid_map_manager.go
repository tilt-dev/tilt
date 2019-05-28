package engine

import (
	"context"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

type UIDMapManager struct {
	watching bool
	kCli     k8s.Client
}

func NewUIDMapManager(kCli k8s.Client) *UIDMapManager {
	return &UIDMapManager{kCli: kCli}
}

func (m *UIDMapManager) needsWatch(st store.RStore) bool {
	if !k8sEventsFeatureFlagOn() {
		return false
	}

	state := st.RLockState()
	defer st.RUnlockState()

	atLeastOneK8S := false
	for _, m := range state.Manifests() {
		if m.IsK8s() {
			atLeastOneK8S = true
		}
	}
	return atLeastOneK8S && state.WatchFiles && !m.watching
}

func (m *UIDMapManager) OnChange(ctx context.Context, st store.RStore) {
	if !m.needsWatch(st) {
		return
	}
	m.watching = true

	ch, err := m.kCli.WatchEverything(ctx, []model.LabelPair{k8s.TiltRunLabel()})
	if err != nil {
		err = errors.Wrap(err, "Error watching for uids. Are you connected to kubernetes?\n")
		st.Dispatch(NewErrorAction(err))
		return
	}

	go m.dispatchUIDsLoop(ctx, ch, st)
}

func (m *UIDMapManager) dispatchUIDsLoop(ctx context.Context, ch <-chan watch.Event, st store.RStore) {
	for {
		select {
		case <-ctx.Done():
			return
		case e, ok := <-ch:
			if !ok {
				return
			}
			if e.Type == watch.Modified {
				continue
			}
			if e.Object == nil {
				continue
			}
			if e.Object.GetObjectKind() == nil {
				continue
			}
			gvk := e.Object.GetObjectKind().GroupVersionKind()
			ke := k8s.K8sEntity{Obj: e.Object, Kind: &gvk}

			manifestName, ok := ke.Labels()[k8s.ManifestNameLabel]
			if !ok {
				continue
			}

			st.Dispatch(UIDUpdateAction{ke.UID(), e.Type, model.ManifestName(manifestName), ke})
		}
	}
}
