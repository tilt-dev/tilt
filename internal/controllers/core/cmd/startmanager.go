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
// to start their objects when a dependency triggers.
//
// TODO(nick): Currently this uses *Cmd types, but it could be generalized
// to any type with a *StartOn field.
type StartManager struct {
	client client.Client

	// A map to help determine which Cmds to reconcile when a UIButton changes.
	//
	// The first key is the UIButton name. The second key is the Cmd Name.
	uiButtonsToTargets map[types.NamespacedName]map[types.NamespacedName]bool

	mu sync.Mutex
}

func NewStartManager(client client.Client) *StartManager {
	return &StartManager{
		client:             client,
		uiButtonsToTargets: make(map[types.NamespacedName]map[types.NamespacedName]bool),
	}
}

// Register the uibuttons for this command.
func (m *StartManager) handleReconcileRequest(cmdName types.NamespacedName, cmd *Cmd) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// TODO(nick): Delete mappings that don't exist anymore.
	// This doesn't hurt anything right now,
	// it just means we get spurious reconcile events.
	if cmd == nil {
		return
	}

	startOn := cmd.Spec.StartOn
	if startOn == nil {
		return
	}

	for _, b := range startOn.UIButtons {
		bn := types.NamespacedName{Name: b}

		// Record in the uibutton -> cmd map
		bTargets, ok := m.uiButtonsToTargets[bn]
		if !ok {
			bTargets = make(map[types.NamespacedName]bool)
			m.uiButtonsToTargets[bn] = bTargets
		}

		bTargets[cmdName] = true
	}
}

// Fetch the last time a start was requested from this target's dependencies.
func (m *StartManager) lastEventTime(ctx context.Context, startOn *StartOnSpec) (time.Time, error) {
	cur := time.Time{}
	if startOn == nil {
		return cur, nil
	}

	for _, bn := range startOn.UIButtons {
		b := &UIButton{}
		err := m.client.Get(ctx, types.NamespacedName{Name: bn}, b)
		if err != nil {
			return cur, err
		}
		lastEventTime := b.Status.LastClickedAt
		if !lastEventTime.Time.Before(startOn.StartAfter.Time) && lastEventTime.Time.After(cur) {
			cur = lastEventTime.Time
		}
	}
	return cur, nil
}

// Given a UIButton update, return all the targets we need to reconcile.
func (m *StartManager) enqueue(b *UIButton) []reconcile.Request {
	m.mu.Lock()
	defer m.mu.Unlock()

	bn := types.NamespacedName{Name: b.Name}
	result := make([]reconcile.Request, 0, len(m.uiButtonsToTargets))
	for cmd := range m.uiButtonsToTargets[bn] {
		result = append(result, reconcile.Request{NamespacedName: cmd})
	}
	return result
}
