package k8swatch

import (
	"context"
	"sync"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/controllers/apis/cluster"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type ServiceWatcher struct {
	clients *cluster.ClientManager

	mu                sync.RWMutex
	watcherKnownState watcherKnownState
	knownServices     map[clusterUID]*v1.Service
}

func NewServiceWatcher(clients cluster.ClientProvider, cfgNS k8s.Namespace) *ServiceWatcher {
	return &ServiceWatcher{
		clients:           cluster.NewClientManager(clients),
		watcherKnownState: newWatcherKnownState(cfgNS),
		knownServices:     make(map[clusterUID]*v1.Service),
	}
}

func (w *ServiceWatcher) diff(st store.RStore) watcherTaskList {
	state := st.RLockState()
	defer st.RUnlockState()

	return w.watcherKnownState.createTaskList(state)
}

func (w *ServiceWatcher) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) error {
	if summary.IsLogOnly() {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	clusters := w.handleClusterChanges(st, summary)

	taskList := w.diff(st)

	for _, teardown := range taskList.teardownNamespaces {
		watcher, ok := w.watcherKnownState.namespaceWatches[teardown]
		if ok {
			watcher.cancel()
		}
		delete(w.watcherKnownState.namespaceWatches, teardown)
	}

	for _, setup := range taskList.setupNamespaces {
		w.setupWatch(ctx, st, clusters, setup)
	}

	if len(taskList.newUIDs) > 0 {
		w.setupNewUIDs(ctx, st, clusters, taskList.newUIDs)
	}

	return nil
}

func (w *ServiceWatcher) handleClusterChanges(st store.RStore, summary store.ChangeSummary) map[types.NamespacedName]*v1alpha1.Cluster {
	clusters := make(map[types.NamespacedName]*v1alpha1.Cluster)
	state := st.RLockState()
	for k, v := range state.Clusters {
		clusters[types.NamespacedName{Name: k}] = v.DeepCopy()
	}
	st.RUnlockState()

	for clusterNN := range summary.Clusters.Changes {
		c := clusters[clusterNN]
		if c != nil && !w.clients.Refresh(c) {
			// cluster config didn't change
			continue
		}

		// cluster config changed, remove all state so it can be re-built
		for key := range w.knownServices {
			if key.cluster == clusterNN {
				delete(w.knownServices, key)
			}
		}

		w.watcherKnownState.resetStateForCluster(clusterNN)
	}

	return clusters
}

func (w *ServiceWatcher) setupWatch(ctx context.Context, st store.RStore, clusters map[types.NamespacedName]*v1alpha1.Cluster, key clusterNamespace) {
	kCli, err := w.clients.GetK8sClient(clusters[key.cluster])
	if err != nil {
		// ignore errors, if the cluster status changes, the subscriber
		// will be re-run and the namespaces will be picked up again as new
		// since watcherKnownState isn't updated
		return
	}

	ch, err := kCli.WatchServices(ctx, key.namespace)
	if err != nil {
		err = errors.Wrapf(err, "Error watching services. Are you connected to kubernetes?\nTry running `kubectl get services -n %q`", key.namespace)
		st.Dispatch(store.NewErrorAction(err))
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	w.watcherKnownState.namespaceWatches[key] = namespaceWatch{cancel: cancel}

	go w.dispatchServiceChangesLoop(ctx, kCli, key.cluster, ch, st)
}

// When new UIDs are deployed, go through all our known services and dispatch
// new events. This handles the case where we get the Service change event
// before the deploy id shows up in the manifest, which is way more common than
// you would think.
func (w *ServiceWatcher) setupNewUIDs(ctx context.Context, st store.RStore, clusters map[types.NamespacedName]*v1alpha1.Cluster, newUIDs map[clusterUID]model.ManifestName) {
	for uid, mn := range newUIDs {
		kCli, err := w.clients.GetK8sClient(clusters[uid.cluster])
		if err != nil {
			// ignore errors, if the cluster status changes, the subscriber
			// will be re-run and the namespaces will be picked up again as new
			// since watcherKnownState isn't updated
			continue
		}

		w.watcherKnownState.knownDeployedUIDs[uid] = mn

		service, ok := w.knownServices[uid]
		if !ok {
			continue
		}

		err = DispatchServiceChange(st, service, mn, kCli.NodeIP(ctx))
		if err != nil {
			logger.Get(ctx).Infof("error resolving service url %s: %v", service.Name, err)
		}
	}
}

// Match up the service update to a manifest.
//
// The division between triageServiceUpdate and recordServiceUpdate is a bit artificial,
// but is designed this way to be consistent with PodWatcher and EventWatchManager.
func (w *ServiceWatcher) triageServiceUpdate(clusterNN types.NamespacedName, service *v1.Service) model.ManifestName {
	w.mu.Lock()
	defer w.mu.Unlock()

	uid := clusterUID{cluster: clusterNN, uid: service.UID}
	w.knownServices[uid] = service

	manifestName, ok := w.watcherKnownState.knownDeployedUIDs[uid]
	if !ok {
		return ""
	}

	return manifestName
}

func (w *ServiceWatcher) dispatchServiceChangesLoop(ctx context.Context, kCli k8s.Client, clusterNN types.NamespacedName, ch <-chan *v1.Service, st store.RStore) {
	for {
		select {
		case service, ok := <-ch:
			if !ok {
				return
			}

			manifestName := w.triageServiceUpdate(clusterNN, service)
			if manifestName == "" {
				continue
			}

			err := DispatchServiceChange(st, service, manifestName, kCli.NodeIP(ctx))
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
