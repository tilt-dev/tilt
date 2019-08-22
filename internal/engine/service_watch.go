package engine

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
	nodeIP       k8s.NodeIP

	mu                sync.RWMutex
	knownDeployedUIDs map[types.UID]model.ManifestName
	knownServices     map[types.UID]*v1.Service
}

func NewServiceWatcher(kCli k8s.Client, ownerFetcher k8s.OwnerFetcher, nodeIP k8s.NodeIP) *ServiceWatcher {
	return &ServiceWatcher{
		kCli:              kCli,
		ownerFetcher:      ownerFetcher,
		nodeIP:            nodeIP,
		knownDeployedUIDs: make(map[types.UID]model.ManifestName),
		knownServices:     make(map[types.UID]*v1.Service),
	}
}

type serviceWatcherTaskList struct {
	needsWatch bool
	newUIDs    map[types.UID]model.ManifestName
}

func (w *ServiceWatcher) diff(st store.RStore) serviceWatcherTaskList {
	state := st.RLockState()
	defer st.RUnlockState()

	w.mu.RLock()
	defer w.mu.RUnlock()

	newUIDs := make(map[types.UID]model.ManifestName)
	atLeastOneK8s := false
	for _, mt := range state.Targets() {
		if !mt.Manifest.IsK8s() {
			continue
		}

		name := mt.Manifest.Name
		atLeastOneK8s = true
		for id := range mt.State.K8sRuntimeState().DeployedUIDSet {
			oldName := w.knownDeployedUIDs[id]
			if name != oldName {
				newUIDs[id] = name
			}
		}
	}

	needsWatch := atLeastOneK8s && state.WatchFiles && !w.watching
	return serviceWatcherTaskList{
		needsWatch: needsWatch,
		newUIDs:    newUIDs,
	}
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

	ch, err := w.kCli.WatchServices(ctx, []model.LabelPair{k8s.TiltRunLabel()})
	if err != nil {
		err = errors.Wrap(err, "Error watching services. Are you connected to kubernetes?\n")
		st.Dispatch(NewErrorAction(err))
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

		err := dispatchServiceChange(st, service, mn, w.nodeIP)
		if err != nil {
			logger.Get(ctx).Infof("error resolving service url %s: %v", service.Name, err)
		}
	}
}

func (w *ServiceWatcher) maybeRecordServiceUpdate(service *v1.Service) (*v1.Service, model.ManifestName) {
	w.mu.Lock()
	defer w.mu.Unlock()

	uid := service.UID
	oldService, ok := w.knownServices[uid]

	// In "real" code, if we get two service updates with the same resource version,
	// we can safely ignore the new one. But dispatching a spurious event
	// in this case makes testing much easier, because the test harness doesn't need
	// to keep track of ResourceVersions
	olderThanKnown := ok && oldService.ResourceVersion > service.ResourceVersion
	if olderThanKnown {
		return nil, ""
	}

	w.knownServices[uid] = service

	manifestName, ok := w.knownDeployedUIDs[uid]
	if !ok {
		return nil, ""
	}

	return service, manifestName
}

func (w *ServiceWatcher) dispatchServiceChangesLoop(ctx context.Context, ch <-chan *v1.Service, st store.RStore) {
	for {
		select {
		case service, ok := <-ch:
			if !ok {
				return
			}

			service, manifestName := w.maybeRecordServiceUpdate(service)
			if service == nil {
				continue
			}

			err := dispatchServiceChange(st, service, manifestName, w.nodeIP)
			if err != nil {
				logger.Get(ctx).Infof("error resolving service url %s: %v", service.Name, err)
			}
		case <-ctx.Done():
			return
		}
	}
}

func dispatchServiceChange(st store.RStore, service *v1.Service, mn model.ManifestName, ip k8s.NodeIP) error {
	url, err := k8s.ServiceURL(service, ip)
	if err != nil {
		return err
	}

	st.Dispatch(NewServiceChangeAction(service, mn, url))
	return nil
}
