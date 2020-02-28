package k8swatch

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

type ServiceWatcher struct {
	kCli         k8s.Client
	ownerFetcher k8s.OwnerFetcher
	watching     bool

	mu                sync.RWMutex
	knownDeployedUIDs map[types.UID]model.ManifestName
	knownServices     map[types.UID]*v1.Service
}

func NewServiceWatcher(kCli k8s.Client, ownerFetcher k8s.OwnerFetcher) *ServiceWatcher {
	return &ServiceWatcher{
		kCli:              kCli,
		ownerFetcher:      ownerFetcher,
		knownDeployedUIDs: make(map[types.UID]model.ManifestName),
		knownServices:     make(map[types.UID]*v1.Service),
	}
}

func (w *ServiceWatcher) diff(st store.RStore) watcherTaskList {
	state := st.RLockState()
	defer st.RUnlockState()

	w.mu.RLock()
	defer w.mu.RUnlock()

	taskList := createWatcherTaskList(state, w.knownDeployedUIDs)
	if w.watching {
		taskList.needsWatch = false
	}
	return taskList
}

func (w *ServiceWatcher) OnChange(ctx context.Context, st store.RStore) {
	taskList := w.diff(st)
	if taskList.needsWatch {
		w.setupWatch(ctx, st)
	}

	if len(taskList.newUIDs) > 0 {
		w.setupNewUIDs(ctx, st, taskList.newUIDs)
	}
}

func (w *ServiceWatcher) setupWatch(ctx context.Context, st store.RStore) {
	w.watching = true

	ch, err := w.kCli.WatchServices(ctx, k8s.ManagedByTiltSelector())
	if err != nil {
		err = errors.Wrap(err, "Error watching services. Are you connected to kubernetes?\n")
		st.Dispatch(store.NewErrorAction(err))
		return
	}

	go w.dispatchServiceChangesLoop(ctx, ch, st)
}

// When new UIDs are deployed, go through all our known services and dispatch
// new events. This handles the case where we get the Service change event
// before the deploy id shows up in the manifest, which is way more common than
// you would think.
func (w *ServiceWatcher) setupNewUIDs(ctx context.Context, st store.RStore, newUIDs map[types.UID]model.ManifestName) {
	w.mu.Lock()
	defer w.mu.Unlock()

	for uid, mn := range newUIDs {
		w.knownDeployedUIDs[uid] = mn

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

	manifestName, ok := w.knownDeployedUIDs[uid]
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
