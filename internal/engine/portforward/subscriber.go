package portforward

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/tilt-dev/tilt/internal/store/k8sconv"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type Subscriber struct {
	kClient k8s.Client

	// Temporary map of PortForward name --> PortForwardEntry, so that we have
	// a way of cancelling running forwards (soon this will be the responsibility
	// of the PortForwardReconciler)
	activeForwards map[string]portForwardEntry
}

func NewSubscriber(kClient k8s.Client) *Subscriber {
	return &Subscriber{
		kClient:        kClient,
		activeForwards: make(map[string]portForwardEntry),
	}
}

// Figure out the diff between what's in the data store and
// what port-forwarding is currently active.
func (s *Subscriber) diff(ctx context.Context, st store.RStore) (toStart []portForwardEntry, toShutdown []portForwardEntry) {
	state := st.RLockState()
	defer st.RUnlockState()

	currentPFs := make(map[string]bool) // 😱 do we want a type for PF Name?

	// Find all the port-forwards that need to be created.
	for _, mt := range state.Targets() {
		ms := mt.State
		manifest := mt.Manifest
		pod := ms.MostRecentPod()
		podID := k8s.PodID(pod.Name)
		if podID.Empty() {
			continue
		}

		// Only do port-forwarding if the pod is running.
		if pod.Phase != string(v1.PodRunning) && !pod.Deleting {
			continue
		}

		forwards := populatePortForwards(manifest, pod)
		if len(forwards) == 0 {
			continue
		}

		for _, fwd := range forwards {
			apiPf := v1alpha1.NewPortForward(fwd.LocalPort, fwd.ContainerPort, fwd.Host, podID.String(), pod.Namespace, ms.Name.String())
			currentPFs[apiPf.Name] = true

			ctx, cancel := context.WithCancel(ctx)
			entry := portForwardEntry{
				PortForward: apiPf,
				ctx:         ctx,
				cancel:      cancel,
			}

			oldEntry, isActive := s.activeForwards[apiPf.Name]
			if isActive {
				// We're already running this port forward, nothing to do
				if equality.Semantic.DeepEqual(oldEntry.Spec, apiPf.Spec) {
					continue
				}

				// The port forward has changed, so remove the old version and re-add the new one
				toShutdown = append(toShutdown, oldEntry)
			}

			toStart = append(toStart, entry)
			state.PortForwards[apiPf.Name] = apiPf
			s.activeForwards[apiPf.Name] = entry
		}
	}

	// Find all the port-forwards that aren't in the manifest anymore
	// or belong to old pods--these need to be shut down.
	for pfName, entry := range s.activeForwards {
		_, isCurrent := currentPFs[pfName]
		if isCurrent {
			continue
		}

		toShutdown = append(toShutdown, entry)
		delete(s.activeForwards, pfName)
	}

	return toStart, toShutdown
}

func (s *Subscriber) OnChange(ctx context.Context, st store.RStore, _ store.ChangeSummary) {
	toStart, toShutdown := s.diff(ctx, st)
	for _, entry := range toShutdown {
		entry.cancel()

		// ◽️ dispatch deletion event for PF set on state
		// TODO(maia): delete PF object in API
	}

	for _, entry := range toStart {
		// Treat port-forwarding errors as part of the pod log
		ctx := logger.CtxWithLogHandler(entry.ctx, PodLogActionWriter{
			Store:        st,
			PodID:        k8s.PodID(entry.Spec.PodName),
			ManifestName: model.ManifestName(entry.ObjectMeta.Annotations[v1alpha1.AnnotationManifest]),
		})

		go s.startPortForwardLoop(ctx, entry)

		// ◽️ dispatch creation event for PF set on state
		// TODO(maia): create PF object in API
	}
}

func (s *Subscriber) startPortForwardLoop(ctx context.Context, entry portForwardEntry) {
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
		err := s.onePortForward(ctx, entry)
		if ctx.Err() != nil {
			// If the context was canceled, we're satisfied.
			// Ignore any errors.
			return
		}

		// Otherwise, repeat the loop, maybe logging the error
		if err != nil {
			logger.Get(ctx).Infof("Reconnecting... Error port-forwarding %s (%d -> %d): %v",
				entry.ObjectMeta.Annotations[v1alpha1.AnnotationManifest],
				entry.Spec.LocalPort, entry.Spec.ContainerPort, err)
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

func (s *Subscriber) onePortForward(ctx context.Context, entry portForwardEntry) error {
	ns := k8s.Namespace(entry.Spec.Namespace)
	podID := k8s.PodID(entry.Spec.PodName)

	pf, err := s.kClient.CreatePortForwarder(ctx, ns, podID, entry.Spec.LocalPort, entry.Spec.ContainerPort, entry.Spec.Host)
	if err != nil {
		return err
	}

	err = pf.ForwardPorts()
	if err != nil {
		return err
	}
	return nil
}

var _ store.Subscriber = &Subscriber{}

// NOTE(maia): this struct is temporary, soon this subscriber won't maintain the state of PF's
//   or be responsible for cancelling them, and so will no longer need the context/cancel func.
type portForwardEntry struct {
	*v1alpha1.PortForward
	ctx    context.Context
	cancel func()
}

// Extract the port-forward specs from the manifest. If any of them
// have ContainerPort = 0, populate them with the default port for the pod.
// Quietly drop forwards that we can't populate.
func populatePortForwards(m model.Manifest, pod v1alpha1.Pod) []model.PortForward {
	cPorts := store.AllPodContainerPorts(pod)
	fwds := m.K8sTarget().PortForwards
	forwards := make([]model.PortForward, 0, len(fwds))
	for _, forward := range fwds {
		if forward.ContainerPort == 0 {
			if len(cPorts) == 0 {
				continue
			}

			forward.ContainerPort = int(cPorts[0])
			for _, cPort := range cPorts {
				if int(forward.LocalPort) == int(cPort) {
					forward.ContainerPort = int(cPort)
					break
				}
			}
		}
		forwards = append(forwards, forward)
	}
	return forwards
}

func PortForwardsAreValid(m model.Manifest, pod v1alpha1.Pod) bool {
	expectedFwds := m.K8sTarget().PortForwards
	actualFwds := populatePortForwards(m, pod)
	return len(actualFwds) == len(expectedFwds)
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
