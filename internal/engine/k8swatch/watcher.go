package k8swatch

import (
	"context"

	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Common utility methods for watching kubernetes resources
type watcherTaskList struct {
	watchableNamespaces []k8s.Namespace
	setupNamespaces     []k8s.Namespace
	teardownNamespaces  []k8s.Namespace
	newUIDs             map[types.UID]model.ManifestName
}

type namespaceWatch struct {
	cancel context.CancelFunc
}

type watcherKnownState struct {
	cfgNS             k8s.Namespace
	namespaceWatches  map[k8s.Namespace]namespaceWatch
	knownDeployedUIDs map[types.UID]model.ManifestName
}

func newWatcherKnownState(cfgNS k8s.Namespace) watcherKnownState {
	return watcherKnownState{
		cfgNS:             cfgNS,
		namespaceWatches:  make(map[k8s.Namespace]namespaceWatch),
		knownDeployedUIDs: make(map[types.UID]model.ManifestName),
	}
}

// Diff the contents of the engine state against the deployed UIDs that the
// watcher already knows about, and create a task list of things to do.
//
// Assumes we're holding an RLock on both states.
func (ks *watcherKnownState) createTaskList(state store.EngineState) watcherTaskList {
	newUIDs := make(map[types.UID]model.ManifestName)
	namespaces := make(map[k8s.Namespace]bool)
	for _, mt := range state.Targets() {
		if !mt.Manifest.IsK8s() {
			continue
		}

		name := mt.Manifest.Name

		if state.EngineMode.WatchesRuntime() {
			for _, obj := range mt.Manifest.K8sTarget().ObjectRefs {
				namespace := k8s.Namespace(obj.Namespace)
				if namespace == "" {
					namespace = ks.cfgNS
				}
				if namespace == "" {
					namespace = k8s.DefaultNamespace
				}
				namespaces[namespace] = true
			}
		}

		// Collect all the new UIDs
		for id := range mt.State.K8sRuntimeState().DeployedUIDSet {
			oldName := ks.knownDeployedUIDs[id]
			if name != oldName {
				newUIDs[id] = name
			}
		}
	}

	var watchableNamespaces []k8s.Namespace
	var setupNamespaces []k8s.Namespace
	var teardownNamespaces []k8s.Namespace

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
