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
// to trigger their objects when a file changes.
//
// TODO(nick): Currently this uses *Cmd types, but it could be generalized
// to any type with a *TriggerSpec field.
type TriggerManager struct {
	client client.Client

	// A map to help determine which Cmds to reconcile when a FileWatch changes.
	//
	// The first key is the FileWatch name. The second key is the Cmd Name.
	fileWatchesToTargets map[types.NamespacedName]map[types.NamespacedName]bool

	uiButtonsToTargets map[types.NamespacedName]map[types.NamespacedName]bool

	mu sync.Mutex
}

func NewTriggerManager(client client.Client) *TriggerManager {
	return &TriggerManager{
		client:               client,
		fileWatchesToTargets: make(map[types.NamespacedName]map[types.NamespacedName]bool),
		uiButtonsToTargets:   make(map[types.NamespacedName]map[types.NamespacedName]bool),
	}
}

// Register the file watches for this command.
func (m *TriggerManager) handleReconcileRequest(cmdName types.NamespacedName, cmd *Cmd) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// TODO(nick): Delete mappings that don't exist anymore.
	// This doesn't hurt anything right now,
	// it just means we get spurious reconcile events.
	if cmd == nil {
		return
	}

	restartOn := cmd.Spec.RestartOn
	startOn := cmd.Spec.StartOn

	filewatches := []string{}
	buttons := []string{}
	if restartOn != nil {
		filewatches = append(filewatches, restartOn.FileWatches...)
		buttons = append(buttons, restartOn.UIButtons...)
	}
	if startOn != nil {
		filewatches = append(filewatches, startOn.FileWatches...)
		buttons = append(buttons, startOn.UIButtons...)
	}

	for _, fw := range filewatches {
		fwn := types.NamespacedName{Name: fw}

		// Record in the filewatch -> cmd map
		fwTargets, ok := m.fileWatchesToTargets[fwn]
		if !ok {
			fwTargets = make(map[types.NamespacedName]bool)
			m.fileWatchesToTargets[fwn] = fwTargets
		}

		fwTargets[cmdName] = true
	}

	for _, b := range buttons {
		bn := types.NamespacedName{Name: b}

		// Record in the button -> cmd map
		bTargets, ok := m.uiButtonsToTargets[bn]
		if !ok {
			bTargets = make(map[types.NamespacedName]bool)
			m.uiButtonsToTargets[bn] = bTargets
		}

		bTargets[cmdName] = true
	}
}

// Fetch the last time a trigger was requested from the given trigger spec
func (m *TriggerManager) lastEventTime(ctx context.Context, triggerOn *TriggerSpec) (time.Time, error) {
	cur := time.Time{}
	if triggerOn == nil {
		return cur, nil
	}

	for _, fwn := range triggerOn.FileWatches {
		fw := &FileWatch{}
		err := m.client.Get(ctx, types.NamespacedName{Name: fwn}, fw)
		if err != nil {
			return cur, err
		}
		lastEventTime := fw.Status.LastEventTime
		if lastEventTime.Time.After(cur) {
			cur = lastEventTime.Time
		}
	}

	for _, bn := range triggerOn.UIButtons {
		b := &UIButton{}
		err := m.client.Get(ctx, types.NamespacedName{Name: bn}, b)
		if err != nil {
			return cur, err
		}
		lastEventTime := b.Status.LastClickedAt
		if lastEventTime.Time.After(cur) {
			cur = lastEventTime.Time
		}
	}
	return cur, nil
}

// Given a FileWatch update, return all the targets we need to reconcile.
func (m *TriggerManager) enqueue(o client.Object) []reconcile.Request {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := types.NamespacedName{Name: o.GetName()}
	var triggerToTarget map[types.NamespacedName]map[types.NamespacedName]bool
	switch o.(type) {
	case *UIButton:
		triggerToTarget = m.uiButtonsToTargets
	case *FileWatch:
		triggerToTarget = m.fileWatchesToTargets
	default:
		return nil
	}

	result := make([]reconcile.Request, 0, len(triggerToTarget))
	for cmd := range triggerToTarget[name] {
		result = append(result, reconcile.Request{NamespacedName: cmd})
	}
	return result
}
