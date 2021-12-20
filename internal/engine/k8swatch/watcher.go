package k8swatch

import (
	"context"

	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

type clusterNamespace struct {
	cluster   types.NamespacedName
	namespace k8s.Namespace
}

type clusterUID struct {
	cluster types.NamespacedName
	uid     types.UID
}

// Common utility methods for watching kubernetes resources
type watcherTaskList struct {
	watchableNamespaces []clusterNamespace
	setupNamespaces     []clusterNamespace
	teardownNamespaces  []clusterNamespace
	newUIDs             map[clusterUID]model.ManifestName
}

type namespaceWatch struct {
	cancel context.CancelFunc
}

type watcherKnownState struct {
	cfgNS             k8s.Namespace
	namespaceWatches  map[clusterNamespace]namespaceWatch
	knownDeployedUIDs map[clusterUID]model.ManifestName
}

func newWatcherKnownState(cfgNS k8s.Namespace) watcherKnownState {
	return watcherKnownState{
		cfgNS:             cfgNS,
		namespaceWatches:  make(map[clusterNamespace]namespaceWatch),
		knownDeployedUIDs: make(map[clusterUID]model.ManifestName),
	}
}

// Diff the contents of the engine state against the deployed UIDs that the
// watcher already knows about, and create a task list of things to do.
//
// Assumes we're holding an RLock on both states.
func (ks *watcherKnownState) createTaskList(state store.EngineState) watcherTaskList {
	newUIDs := make(map[clusterUID]model.ManifestName)
	seenUIDs := make(map[clusterUID]bool)
	namespaces := make(map[clusterNamespace]bool)
	for _, mt := range state.Targets() {
		if !mt.Manifest.IsK8s() {
			continue
		}

		// TODO(milas): read the Cluster object name from the spec once available
		clusterNN := types.NamespacedName{Name: v1alpha1.ClusterNameDefault}

		name := mt.Manifest.Name

		// Collect all the new UIDs
		applyFilter := mt.State.K8sRuntimeState().ApplyFilter
		if applyFilter != nil {
			for _, ref := range applyFilter.DeployedRefs {
				namespace := k8s.Namespace(ref.Namespace)
				if namespace == "" {
					namespace = ks.cfgNS
				}
				if namespace == "" {
					namespace = k8s.DefaultNamespace
				}
				nsKey := clusterNamespace{cluster: clusterNN, namespace: namespace}
				namespaces[nsKey] = true

				// Our data model allows people to have the same resource defined in
				// multiple manifests, and so we can have the same deployed UID in
				// multiple manifests.
				//
				// This check protects us from infinite loops where the diff keeps flipping
				// between the two manifests.
				//
				// Ideally, our data model would prevent this from happening entirely.
				uidKey := clusterUID{cluster: clusterNN, uid: ref.UID}
				if seenUIDs[uidKey] {
					continue
				}
				seenUIDs[uidKey] = true

				oldName := ks.knownDeployedUIDs[uidKey]
				if name != oldName {
					newUIDs[uidKey] = name
				}
			}
		}
	}

	// If we're no longer deploying a manifest, delete it from the known deployed UIDs.
	// This ensures that if it shows up again, we process it correctly.
	for uid := range ks.knownDeployedUIDs {
		if !seenUIDs[uid] {
			delete(ks.knownDeployedUIDs, uid)
		}
	}

	var watchableNamespaces []clusterNamespace
	var setupNamespaces []clusterNamespace
	var teardownNamespaces []clusterNamespace

	for needed := range namespaces {
		watchableNamespaces = append(watchableNamespaces, needed)
		if _, ok := ks.namespaceWatches[needed]; !ok {
			setupNamespaces = append(setupNamespaces, needed)
		}
	}

	for existing := range ks.namespaceWatches {
		if _, ok := namespaces[existing]; !ok {
			teardownNamespaces = append(teardownNamespaces, existing)
		}
	}

	return watcherTaskList{
		watchableNamespaces: watchableNamespaces,
		setupNamespaces:     setupNamespaces,
		teardownNamespaces:  teardownNamespaces,
		newUIDs:             newUIDs,
	}
}
