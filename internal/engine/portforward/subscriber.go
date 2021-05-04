package portforward

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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

	// We store active forwards here so that we have a way of canceling running
	// forwards--soon this will be the responsibility of the PortForwardReconciler
	activeForwards map[k8s.PodID]portForwardEntry
}

func NewSubscriber(kClient k8s.Client) *Subscriber {
	return &Subscriber{
		kClient:        kClient,
		activeForwards: make(map[k8s.PodID]portForwardEntry),
	}
}

// Figure out the diff between what's in the data store and
// what port-forwarding is currently active.
func (s *Subscriber) diff(ctx context.Context, st store.RStore) (toStart []portForwardEntry, toShutdown []portForwardEntry) {
	state := st.RLockState()
	defer st.RUnlockState()

	statePods := make(map[k8s.PodID]bool, len(state.ManifestTargets))

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

		statePods[podID] = true

		apiPf := &v1alpha1.PortForward{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("port-forward-%s", podID),
				Annotations: map[string]string{
					// Name of the manifest that this Port Forward corresponds to
					// (we need this to route the logs correctly)
					v1alpha1.AnnotationManifest: manifest.Name.String(),
					v1alpha1.AnnotationSpanID:   string(k8sconv.SpanIDForPod(podID)),
				},
			},
			Spec: PortForwardSpec{
				PodName:   podID.String(),
				Namespace: pod.Namespace,
				Forwards:  modelForwardsToApiForwards(forwards),
			},
		}

		ctx, cancel := context.WithCancel(ctx)
		entry := portForwardEntry{
			PortForward: apiPf,
			ctx:         ctx,
			cancel:      cancel,
		}

		oldEntry, isActive := s.activeForwards[podID]
		if isActive {
			if equality.Semantic.DeepEqual(oldEntry.Spec, apiPf.Spec) {
				// We're already running this port forward, nothing to do
				continue
			}

			// Tear down the old version so we can start the new one
			toShutdown = append(toShutdown, oldEntry)
		}

		toStart = append(toStart, entry)
		s.activeForwards[podID] = entry
	}

	// Find all the port-forwards that belong to old pods--these need to be shut down.
	for podID, entry := range s.activeForwards {
		_, inState := statePods[podID]
		if inState {
			continue
		}

		toShutdown = append(toShutdown, entry)
		delete(s.activeForwards, podID)
	}

	return toStart, toShutdown
}

func (s *Subscriber) OnChange(ctx context.Context, st store.RStore, _ store.ChangeSummary) {
	toStart, toShutdown := s.diff(ctx, st)
	for _, entry := range toShutdown {
		entry.cancel()

		// TODO(maia): st.Dispatch(NewPortForwardDeleteAction(entry.Name))
		// TODO(maia): delete PF object in API
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
			go s.startPortForwardLoop(ctx, entry, forward)
		}

		// TODO(maia): st.Dispatch(NewPortForwardCreateAction(entry.PortForward))
		// TODO(maia): create PF object in API
	}
}

func (s *Subscriber) startPortForwardLoop(ctx context.Context, entry portForwardEntry, forward v1alpha1.Forward) {
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
		err := s.onePortForward(ctx, entry, forward)
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

func (s *Subscriber) onePortForward(ctx context.Context, entry portForwardEntry, forward v1alpha1.Forward) error {
	ns := k8s.Namespace(entry.Spec.Namespace)
	podID := k8s.PodID(entry.Spec.PodName)

	pf, err := s.kClient.CreatePortForwarder(ctx, ns, podID, int(forward.LocalPort), int(forward.ContainerPort), forward.Host)
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
//   or be responsible for canceling them
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
	w.Store.Dispatch(store.NewLogAction(w.ManifestName, k8sconv.SpanIDForPod(w.ManifestName, w.PodID), level, fields, p))
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
