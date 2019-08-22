package engine

import (
	"context"
	"sync"
	"time"

	"github.com/jonboulle/clockwork"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"
)

// no explicit analysis went into the selection of these numbers
const uidMapEntryTTL = 10 * time.Minute
const uidMapJanitorInterval = 3 * time.Minute

type uidMapEntry struct {
	resourceVersion   string
	manifest          model.ManifestName
	obj               k8s.K8sEntity
	belongsToThisTilt bool
	expiresAt         time.Time
}

type EventWatchManager struct {
	kClient  k8s.Client
	watching bool
	uidMap   map[types.UID]uidMapEntry
	uidMapMu sync.RWMutex
	clock    clockwork.Clock
}

func NewEventWatchManager(kClient k8s.Client, clock clockwork.Clock) *EventWatchManager {
	return &EventWatchManager{
		kClient: kClient,
		uidMap:  make(map[types.UID]uidMapEntry),
		clock:   clock,
	}
}

func (m *EventWatchManager) needsWatch(st store.RStore) bool {
	state := st.RLockState()
	defer st.RUnlockState()

	atLeastOneK8s := false
	for _, m := range state.Manifests() {
		if m.IsK8s() {
			atLeastOneK8s = true
		}
	}
	return atLeastOneK8s && state.WatchFiles && !m.watching
}

func (m *EventWatchManager) OnChange(ctx context.Context, st store.RStore) {
	if !m.needsWatch(st) {
		return
	}

	m.watching = true

	ch, err := m.kClient.WatchEvents(ctx)
	if err != nil {
		err = errors.Wrap(err, "Error watching k8s events\n")
		st.Dispatch(NewErrorAction(err))
		return
	}

	go m.dispatchEventsLoop(ctx, ch, st)
}

func (m *EventWatchManager) createEntry(ctx context.Context, involvedObject v1.ObjectReference) uidMapEntry {
	ret := uidMapEntry{
		resourceVersion:   involvedObject.ResourceVersion,
		belongsToThisTilt: false,
		expiresAt:         m.clock.Now().Add(uidMapEntryTTL),
	}

	e, err := m.kClient.GetByReference(ctx, involvedObject)
	if err != nil {
		// if the lookup was an error, wipe out resourceVersion so that we don't cache a potentially
		// ephemeral negative result
		// (unfortunately, this means we won't log the event for which this lookup failed)
		ret.resourceVersion = "0"
		return ret
	}

	mn := model.ManifestName(e.Labels()[k8s.ManifestNameLabel])
	if mn == "" {
		return ret
	}

	ret.obj = e
	ret.manifest = mn
	ret.belongsToThisTilt = e.Labels()[k8s.TiltRunIDLabel] == k8s.TiltRunID
	return ret
}

// This does not attempt to prevent duplicate simultaneous requests. If we get multiple events for the same
// object at the same time, they can each result in their own API request.
// We're currently assuming this matters sufficiently rarely that it's not worth the code complexity to fix.
func (m *EventWatchManager) getEntry(ctx context.Context, obj v1.ObjectReference) uidMapEntry {
	uid := obj.UID

	m.uidMapMu.RLock()
	entry, ok := m.uidMap[uid]
	m.uidMapMu.RUnlock()
	if !ok || entry.resourceVersion != obj.ResourceVersion {
		entry = m.createEntry(ctx, obj)
		m.uidMapMu.Lock()
		// another thread might have come in and set this to the same or even a newer value by now
		// neither of these cases should affect behavior aside from causing some unneeded api requests
		m.uidMap[uid] = entry
		m.uidMapMu.Unlock()
	}

	return entry
}

// just loops and deletes any entrys that have hit their expiration
func (m *EventWatchManager) uidMapJanitor(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-m.clock.After(uidMapJanitorInterval):
			m.uidMapMu.Lock()
			for k, v := range m.uidMap {
				if m.clock.Now().After(v.expiresAt) {
					delete(m.uidMap, k)
				}
			}
			m.uidMapMu.Unlock()
		}
	}
}

func (m *EventWatchManager) dispatchEventsLoop(ctx context.Context, ch <-chan *v1.Event, st store.RStore) {
	go m.uidMapJanitor(ctx)

	state := st.RLockState()
	tiltStartTime := state.TiltStartTime
	st.RUnlockState()

	for {
		select {
		case event, ok := <-ch:
			if !ok {
				return
			}

			// on startup, k8s will give us a bunch of event objects that happened before tilt started, which
			// leads to flooding the k8s api with lookups on those events' involvedObjects
			// we don't care about those events, so ignore them.
			if event.ObjectMeta.CreationTimestamp.Time.Before(tiltStartTime) {
				continue
			}

			go func() {
				entry := m.getEntry(ctx, event.InvolvedObject)

				if entry.belongsToThisTilt {
					st.Dispatch(store.NewK8sEventAction(event, entry.manifest, entry.obj))
				}
			}()

		case <-ctx.Done():
			return
		}
	}
}
