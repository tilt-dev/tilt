package k8swatch

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type ServiceWatcher struct {
	kCli k8s.Client

	mu                sync.RWMutex
	watcherKnownState watcherKnownState
	knownServices     map[types.UID]*v1.Service
}

func NewServiceWatcher(kCli k8s.Client, cfgNS k8s.Namespace) *ServiceWatcher {
	return &ServiceWatcher{
		kCli:              kCli,
		watcherKnownState: newWatcherKnownState(cfgNS),
		knownServices:     make(map[types.UID]*v1.Service),
	}
}

func (w *ServiceWatcher) diff(st store.RStore) watcherTaskList {
	state := st.RLockState()
	defer st.RUnlockState()

	w.mu.RLock()
	defer w.mu.RUnlock()

	return w.watcherKnownState.createTaskList(state)
}

func (w *ServiceWatcher) OnChange(ctx context.Context, st store.RStore, _ store.ChangeSummary) error {
	taskList := w.diff(st)

	w.mu.Lock()
	defer w.mu.Unlock()

	for _, teardown := range taskList.teardownNamespaces {
		watcher, ok := w.watcherKnownState.namespaceWatches[teardown]
		if ok {
			watcher.cancel()
		}
		delete(w.watcherKnownState.namespaceWatches, teardown)
	}

	for _, setup := range taskList.setupNamespaces {
		w.setupWatch(ctx, st, setup)
	}

	if len(taskList.newUIDs) > 0 {
		w.setupNewUIDs(ctx, st, taskList.newUIDs)
	}

	return nil
}

func (w *ServiceWatcher) setupWatch(ctx context.Context, st store.RStore, ns k8s.Namespace) {
	ch, err := w.kCli.WatchServices(ctx, ns)
	if err != nil {
		err = errors.Wrapf(err, "Error watching services. Are you connected to kubernetes?\nTry running `kubectl get services -n %q`", ns)
		st.Dispatch(store.NewErrorAction(err))
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	w.watcherKnownState.namespaceWatches[ns] = namespaceWatch{cancel: cancel}

	go w.dispatchServiceChangesLoop(ctx, ch, st)
}

// When new UIDs are deployed, go through all our known services and dispatch
// new events. This handles the case where we get the Service change event
// before the deploy id shows up in the manifest, which is way more common than
// you would think.
func (w *ServiceWatcher) setupNewUIDs(ctx context.Context, st store.RStore, newUIDs map[types.UID]model.ManifestName) {
	for uid, mn := range newUIDs {
		w.watcherKnownState.knownDeployedUIDs[uid] = mn

		service, ok := w.knownServices[uid]
		if !ok {
			continue
		}

		err := DispatchServiceChange(st, service, mn, w.kCli.NodeIP(ctx))
		if err != nil {
			logger.Get(ctx).Infof("error resolving service url %s: %v", service.Name, err)
		}
	}
}

// Match up the service update to a manifest.
//
// The division between triageServiceUpdate and recordServiceUpdate is a bit artificial,
// but is designed this way to be consistent with PodWatcher and EventWatchManager.
func (w *ServiceWatcher) triageServiceUpdate(service *v1.Service) model.ManifestName {
	w.mu.Lock()
	defer w.mu.Unlock()

	uid := service.UID
	w.knownServices[uid] = service

	manifestName, ok := w.watcherKnownState.knownDeployedUIDs[uid]
	if !ok {
		return ""
	}

	return manifestName
}

func (w *ServiceWatcher) dispatchServiceChangesLoop(ctx context.Context, ch <-chan *v1.Service, st store.RStore) {
	for {
		select {
		case service, ok := <-ch:
			if !ok {
				return
			}

			manifestName := w.triageServiceUpdate(service)
			if manifestName == "" {
				continue
			}

			err := DispatchServiceChange(st, service, manifestName, w.kCli.NodeIP(ctx))
			if err != nil {
				logger.Get(ctx).Infof("error resolving service url %s: %v", service.Name, err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func DispatchServiceChange(st store.RStore, service *v1.Service, mn model.ManifestName, ip k8s.NodeIP) error {
	url, err := k8s.ServiceURL(service, ip)
	if err != nil {
		return err
	}

	st.Dispatch(NewServiceChangeAction(service, mn, url))
	return nil
}
