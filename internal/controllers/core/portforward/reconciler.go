package portforward

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/tilt-dev/tilt/internal/store/k8sconv"

	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type Reconciler struct {
	kClient k8s.Client

	// map of PortForward object name --> running forward(s)
	activeForwards map[string]portForwardEntry
}

func NewSubscriber(kClient k8s.Client) *Reconciler {
	return &Reconciler{
		kClient:        kClient,
		activeForwards: make(map[string]portForwardEntry),
	}
}

// Figure out the diff between what's in the data store and
// what port-forwarding is currently active.
func (r *Reconciler) diff(ctx context.Context, changedPFs store.ChangeSet, st store.RStore) (toStart []portForwardEntry, toShutdown []portForwardEntry) {
	state := st.RLockState()
	defer st.RUnlockState()

	statePFs := state.PortForwards
	for pfName := range changedPFs.Changes {
		desired, onState := statePFs[pfName.String()]
		existing, isActive := r.activeForwards[pfName.String()]
		if !onState {
			// This port forward is no longer on the state; the fact that we got a
			// change event indicates that it's been deleted, shut it down.
			if !isActive {
				// This isn't great but idk if we want to freak out or just let it happen?
				panic(fmt.Sprintf("couldn't find running port forward %s even tho it was just deleted from state?!", pfName.String()))
			}
			toShutdown = append(toShutdown, existing)
			continue
		}

		if isActive {
			// We're already running this PortForward -- do we need to do anything further?
			if equality.Semantic.DeepEqual(existing.Spec, desired.Spec) && equality.Semantic.DeepEqual(existing.ObjectMeta, desired.ObjectMeta) {
				// Nothing has changed, nothing to do
				continue
			}

			// There's been a change to the spec for this PortForward, so tear down the old version
			toShutdown = append(toShutdown, existing)
		}

		// We're not running this PortForward(/the current version of this PortForward), so spin it up
		toStart = append(toStart, newEntry(ctx, desired))
	}

	return toStart, toShutdown
}

func (r *Reconciler) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) {
	if summary.IsLogOnly() {
		return
	}

	toStart, toShutdown := r.diff(ctx, summary.PortForwards, st)
	for _, entry := range toShutdown {
		entry.cancel()
	}

	for _, entry := range toStart {
		// Treat port-forwarding errors as part of the pod log
		ctx := logger.CtxWithLogHandler(entry.ctx, PodLogActionWriter{
			Store:        st,
			PodID:        k8s.PodID(entry.Spec.PodName),
			ManifestName: model.ManifestName(entry.ObjectMeta.Annotations[v1alpha1.AnnotationManifest]),
		})

		for _, forward := range entry.Spec.Forwards {
			entry := entry
			forward := forward
			go r.startPortForwardLoop(ctx, entry, forward)
		}
	}
}

func (r *Reconciler) startPortForwardLoop(ctx context.Context, entry portForwardEntry, forward v1alpha1.Forward) {
	originalBackoff := wait.Backoff{
		Steps:    1000,
		Duration: 50 * time.Millisecond,
		Factor:   2.0,
		Jitter:   0.1,
		Cap:      15 * time.Second,
	}
	currentBackoff := originalBackoff

	for {
		start := time.Now()
		err := r.onePortForward(ctx, entry, forward)
		if ctx.Err() != nil {
			// If the context was canceled, we're satisfied.
			// Ignore any errors.
			return
		}

		// Otherwise, repeat the loop, maybe logging the error
		if err != nil {
			logger.Get(ctx).Infof("Reconnecting... Error port-forwarding %s (%d -> %d): %v",
				entry.ObjectMeta.Annotations[v1alpha1.AnnotationManifest],
				forward.LocalPort, forward.ContainerPort, err)
		}

		// If this failed in less than a second, then we should advance the backoff.
		// Otherwise, reset the backoff.
		if time.Since(start) < time.Second {
			time.Sleep(currentBackoff.Step())
		} else {
			currentBackoff = originalBackoff
		}
	}
}

func (r *Reconciler) onePortForward(ctx context.Context, entry portForwardEntry, forward v1alpha1.Forward) error {
	ns := k8s.Namespace(entry.Spec.Namespace)
	podID := k8s.PodID(entry.Spec.PodName)

	pf, err := r.kClient.CreatePortForwarder(ctx, ns, podID, int(forward.LocalPort), int(forward.ContainerPort), forward.Host)
	if err != nil {
		return err
	}

	err = pf.ForwardPorts()
	if err != nil {
		return err
	}
	return nil
}

var _ store.Subscriber = &Reconciler{}

type portForwardEntry struct {
	*v1alpha1.PortForward
	ctx    context.Context
	cancel func()
}

func newEntry(ctx context.Context, pf *PortForward) portForwardEntry {
	ctx, cancel := context.WithCancel(ctx)
	return portForwardEntry{
		PortForward: pf,
		ctx:         ctx,
		cancel:      cancel,
	}
}

type PodLogActionWriter struct {
	Store        store.RStore
	PodID        k8s.PodID
	ManifestName model.ManifestName
}

func (w PodLogActionWriter) Write(level logger.Level, fields logger.Fields, p []byte) error {
	w.Store.Dispatch(store.NewLogAction(w.ManifestName, k8sconv.SpanIDForPod(w.PodID), level, fields, p))
	return nil
}

func modelForwardsToApiForwards(forwards []model.PortForward) []v1alpha1.Forward {
	res := make([]v1alpha1.Forward, len(forwards))
	for i, fwd := range forwards {
		res[i] = v1alpha1.Forward{
			LocalPort:     int32(fwd.LocalPort),
			ContainerPort: int32(fwd.ContainerPort),
			Host:          fwd.Host,
		}
	}
	return res
}
