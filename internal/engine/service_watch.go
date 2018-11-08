package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/store"

	"github.com/windmilleng/tilt/internal/k8s"
	"k8s.io/api/core/v1"
)

type ServiceWatcher struct {
	kCli     k8s.Client
	watching bool
	nodeIP   k8s.NodeIP
}

func NewServiceWatcher(kCli k8s.Client, nodeIP k8s.NodeIP) *ServiceWatcher {
	return &ServiceWatcher{
		kCli:   kCli,
		nodeIP: nodeIP,
	}
}

func (w *ServiceWatcher) needsWatch(sr store.StateReader) bool {
	state := sr.RLockState()
	defer sr.RUnlockState()
	return state.WatchMounts && !w.watching
}

func (w *ServiceWatcher) OnChange(ctx context.Context, dsr store.DispatchingStateReader) {
	if !w.needsWatch(dsr) {
		return
	}
	w.watching = true

	ch, err := w.kCli.WatchServices(ctx, []k8s.LabelPair{TiltRunLabel()})
	if err != nil {
		dsr.Dispatch(NewErrorAction(err))
		return
	}

	go w.dispatchServiceChangesLoop(ctx, ch, dsr)
}

func (w *ServiceWatcher) dispatchServiceChangesLoop(ctx context.Context, ch <-chan *v1.Service, d store.Dispatcher) {
	for {
		select {
		case service, ok := <-ch:
			if !ok {
				return
			}

			err := dispatchServiceChange(d, service, w.nodeIP)
			if err != nil {
				logger.Get(ctx).Infof("error resolving service url %s: %v", service.Name, err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func dispatchServiceChange(d store.Dispatcher, service *v1.Service, ip k8s.NodeIP) error {
	url, err := k8s.ServiceURL(service, ip)
	if err != nil {
		return err
	}

	d.Dispatch(NewServiceChangeAction(service, url))
	return nil
}
