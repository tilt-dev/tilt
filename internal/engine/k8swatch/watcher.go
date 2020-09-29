package k8swatch

import (
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Common utility methods for watching kubernetes resources
type watcherTaskList struct {
	needsWatch bool
	newUIDs    map[types.UID]model.ManifestName
}

type watcherKnownState struct {
	knownDeployedUIDs map[types.UID]model.ManifestName
}

func newWatcherKnownState() watcherKnownState {
	return watcherKnownState{
		knownDeployedUIDs: make(map[types.UID]model.ManifestName),
	}
}

// Diff the contents of the engine state against the deployed UIDs that the
// watcher already knows about, and create a task list of things to do.
//
// Assumes we're holding an RLock on both states.
func (ks *watcherKnownState) createTaskList(state store.EngineState) watcherTaskList {
	newUIDs := make(map[types.UID]model.ManifestName)
	atLeastOneK8s := false
	for _, mt := range state.Targets() {
		if !mt.Manifest.IsK8s() {
			continue
		}

		name := mt.Manifest.Name
		atLeastOneK8s = true

		// Collect all the new UIDs
		for id := range mt.State.K8sRuntimeState().DeployedUIDSet {
			oldName := ks.knownDeployedUIDs[id]
			if name != oldName {
				newUIDs[id] = name
			}
		}
	}

	needsWatch := atLeastOneK8s && state.EngineMode.WatchesRuntime()
	return watcherTaskList{
		needsWatch: needsWatch,
		newUIDs:    newUIDs,
	}
}

// Updates the known state when we've completed the task list.
//
// Assumes we're holding a lock on this state.
func (ks *watcherKnownState) finishTaskList(l watcherTaskList) {
	for uid, mn := range l.newUIDs {
		ks.knownDeployedUIDs[uid] = mn
	}
}
