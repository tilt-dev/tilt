package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/store"

	"github.com/windmilleng/tilt/internal/k8s"
	"k8s.io/api/core/v1"
)

type ServiceWatcher struct {
	kCli     k8s.Client
	watching bool
}

func NewServiceWatcher(kCli k8s.Client) *ServiceWatcher {
	return &ServiceWatcher{
		kCli: kCli,
	}
}

func (w *ServiceWatcher) needsWatch(st *store.Store) bool {
	state := st.RLockState()
	defer st.RUnlockState()
	return state.WatchMounts && !w.watching
}

func (w *ServiceWatcher) OnChange(ctx context.Context, st *store.Store) {
	if !w.needsWatch(st) {
		return
	}
	w.watching = true

	ch, err := w.kCli.WatchServices(ctx, []k8s.LabelPair{TiltRunLabel()})
	if err != nil {
		st.Dispatch(NewErrorAction(err))
		return
	}

	go dispatchServiceChangesLoop(ctx, ch, st)
}

func dispatchServiceChangesLoop(ctx context.Context, ch <-chan *v1.Service, st *store.Store) {
	for {
		select {
		case service, ok := <-ch:
			if !ok {
				return
			}
			st.Dispatch(NewServiceChangeAction(service))
		case <-ctx.Done():
			return
		}
	}
}
