package cmd

import (
	"context"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// Helper struct to help reconcilers determine when
// to restart their objects when a file changes.
//
// TODO(nick): Currently this uses *Cmd types, but it could be generalized
// to any type with a *RestartOn field.
type RestartManager struct {
	client client.Client

	// A map to help determine which Cmds to reconcile when a FileWatch changes.
	//
	// The first key is the FileWatch name. The second key is the Cmd Name.
	fileWatchesToTargets map[types.NamespacedName]map[types.NamespacedName]bool

	mu sync.Mutex
}

func NewRestartManager(client client.Client) *RestartManager {
	return &RestartManager{
		client:               client,
		fileWatchesToTargets: make(map[types.NamespacedName]map[types.NamespacedName]bool),
	}
}

// Register the file watches for this command.
func (m *RestartManager) handleReconcileRequest(cmdName types.NamespacedName, cmd *Cmd) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// TODO(nick): Delete mappings that don't exist anymore.
	// This doesn't hurt anything right now,
	// it just means we get spurious reconcile events.
	if cmd == nil {
		return
	}

	restartOn := cmd.Spec.RestartOn
	if restartOn == nil {
		return
	}

	for _, fw := range cmd.Spec.RestartOn.FileWatches {
		fwn := types.NamespacedName{Name: fw}

		// Record in the filewatch -> cmd map
		fwTargets, ok := m.fileWatchesToTargets[fwn]
		if !ok {
			fwTargets = make(map[types.NamespacedName]bool)
			m.fileWatchesToTargets[fwn] = fwTargets
		}

		fwTargets[cmdName] = true
	}
}

// Fetch the last time a restart was requested from this target's dependencies.
func (m *RestartManager) lastEventTime(ctx context.Context, restartOn *RestartOnSpec) (time.Time, error) {
	cur := time.Time{}
	if restartOn == nil {
		return cur, nil
	}

	for _, fwn := range restartOn.FileWatches {
		fw := &FileWatch{}
		err := m.client.Get(ctx, types.NamespacedName{Name: fwn}, fw)
		if err != nil {
			return cur, err
		}
		lastEventTime := fw.Status.LastEventTime
		if lastEventTime != nil {
			if lastEventTime.Time.After(cur) {
				cur = lastEventTime.Time
			}
		}
	}
	return cur, nil
}

// Given a FileWatch update, return all the targets we need to reconcile.
func (m *RestartManager) enqueue(fw *FileWatch) []reconcile.Request {
	m.mu.Lock()
	defer m.mu.Unlock()

	fwn := types.NamespacedName{Name: fw.Name}
	result := make([]reconcile.Request, 0, len(m.fileWatchesToTargets))
	for cmd := range m.fileWatchesToTargets[fwn] {
		result = append(result, reconcile.Request{NamespacedName: cmd})
	}
	return result
}
