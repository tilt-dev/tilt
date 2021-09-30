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
	seenUIDs := make(map[types.UID]bool)
	namespaces := make(map[k8s.Namespace]bool)
	for _, mt := range state.Targets() {
		if !mt.Manifest.IsK8s() {
			continue
		}

		name := mt.Manifest.Name

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

		// Collect all the new UIDs
		applyFilter := mt.State.K8sRuntimeState().ApplyFilter
		if applyFilter != nil {
			for _, ref := range applyFilter.DeployedRefs {
				// Our data model allows people to have the same resource defined in
				// multiple manifests, and so we can have the same deployed UID in
				// multiple manifests.
				//
				// This check protects us from infinite loops where the diff keeps flipping
				// between the two manifests.
				//
				// Ideally, our data model would prevent this from happening entirely.
				id := ref.UID
				if seenUIDs[id] {
					continue
				}
				seenUIDs[id] = true

				oldName := ks.knownDeployedUIDs[id]
				if name != oldName {
					newUIDs[id] = name
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
