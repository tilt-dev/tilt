package engine

import (
	"context"
	"time"

	"github.com/windmilleng/tilt/internal/model"
	v1 "k8s.io/api/core/v1"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/store"

	"github.com/windmilleng/tilt/internal/k8s"
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

func (w *ServiceWatcher) needsWatch(st store.RStore) bool {
	state := st.RLockState()
	defer st.RUnlockState()

	atLeastOneK8S := false
	for _, m := range state.Manifests() {
		if m.IsK8s() {
			atLeastOneK8S = true
		}
	}
	return atLeastOneK8S && state.WatchMounts && !w.watching
}

func (w *ServiceWatcher) OnChange(ctx context.Context, st store.RStore) {
	if !w.needsWatch(st) {
		return
	}
	w.watching = true

	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ch, err := w.kCli.WatchServices(ctx2, []model.LabelPair{k8s.TiltRunLabel()})
	if err != nil {
		err = errors.Wrap(err, "Error watching services. Are you connected to kubernetes?\n")
		st.Dispatch(NewErrorAction(err))
		return
	}

	go w.dispatchServiceChangesLoop(ctx, ch, st)
}

func (w *ServiceWatcher) dispatchServiceChangesLoop(ctx context.Context, ch <-chan *v1.Service, st store.RStore) {
	for {
		select {
		case service, ok := <-ch:
			if !ok {
				return
			}

			err := dispatchServiceChange(st, service, w.nodeIP)
			if err != nil {
				logger.Get(ctx).Infof("error resolving service url %s: %v", service.Name, err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func dispatchServiceChange(st store.RStore, service *v1.Service, ip k8s.NodeIP) error {
	url, err := k8s.ServiceURL(service, ip)
	if err != nil {
		return err
	}

	st.Dispatch(NewServiceChangeAction(service, url))
	return nil
}
