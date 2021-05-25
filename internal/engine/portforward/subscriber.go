package portforward

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"github.com/tilt-dev/tilt/internal/store/k8sconv"

	v1 "k8s.io/api/core/v1"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/model"
)

type Subscriber struct {
	kClient            k8s.Client
	ctrlClient         ctrlclient.Client
	disabledForTesting bool
}

func NewSubscriber(kClient k8s.Client, ctrlClient ctrlclient.Client) *Subscriber {
	return &Subscriber{
		kClient:    kClient,
		ctrlClient: ctrlClient,
	}
}

func (s *Subscriber) DisableForTesting() { s.disabledForTesting = true }

// Figure out the diff between what port forwards ought to be running (given the
// current manifests and pods) and what the EngineState/API think ought to be running
func (s *Subscriber) diff(st store.RStore) (toStart, toShutdown []*PortForward) {
	if s.disabledForTesting {
		return
	}

	state := st.RLockState()
	defer st.RUnlockState()

	statePods := make(map[k8s.PodID]bool, len(state.ManifestTargets))
	statePFs := state.PortForwards
	expectedPFs := map[string]bool{} // names of all the port forwards that should be running

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

		pf := &v1alpha1.PortForward{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("port-forward-%s", podID),
				Annotations: map[string]string{
					// Name of the manifest that this Port Forward corresponds to
					// (we need this to route the logs correctly)
					v1alpha1.AnnotationManifest: manifest.Name.String(),
					v1alpha1.AnnotationSpanID:   string(k8sconv.SpanIDForPod(manifest.Name, podID)),
				},
			},
			Spec: PortForwardSpec{
				PodName:   podID.String(),
				Namespace: pod.Namespace,
				Forwards:  modelForwardsToApiForwards(forwards),
			},
		}

		expectedPFs[pf.Name] = true

		oldPF, onState := statePFs[pf.Name]
		if onState {
			// This PortForward is already on the state -- do we need to do anything further?
			// NOTE(maia): we compare the ManifestName annotation so that if a user changes
			//   just the manifest name, the PF logs will go to the correct place.
			if equality.Semantic.DeepEqual(oldPF.Spec, pf.Spec) &&
				oldPF.ObjectMeta.Annotations[v1alpha1.AnnotationManifest] ==
					pf.ObjectMeta.Annotations[v1alpha1.AnnotationManifest] {
				// Nothing has changed, nothing to do
				continue
			}
			// The port forward needs to be UPDATED--which today is the same as a "create"
			// event, which overwrites the current info for this port forward name
		}
		toStart = append(toStart, pf)
	}

	// Find any PFs on the state that our latest loop doesn't think should exist;
	// these need to be shut down
	for name, pf := range statePFs {
		if !expectedPFs[name] {
			toShutdown = append(toShutdown, pf)
		}
	}

	return toStart, toShutdown
}

func (s *Subscriber) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) {
	if summary.IsLogOnly() {
		return
	}
	toStart, toShutdown := s.diff(st)
	for _, pf := range toShutdown {
		s.deletePF(ctx, st, pf)
	}

	for _, pf := range toStart {
		s.createPF(ctx, st, pf)
	}
}

var _ store.Subscriber = &Subscriber{}

// Delete the PortForward API object. Should be idempotent.
func (s *Subscriber) deletePF(ctx context.Context, st store.RStore, pf *PortForward) {
	err := s.ctrlClient.Delete(ctx, pf)
	if err != nil &&
		!apierrors.IsNotFound(err) {
		st.Dispatch(store.NewErrorAction(fmt.Errorf(
			"deleting PortForward from apiserver: %v", err)))
		return
	}
	st.Dispatch(NewPortForwardDeleteAction(pf.Name))
}

// Create/update a PortForward API object, if necessary. Should be idempotent.
func (s *Subscriber) createPF(ctx context.Context, st store.RStore, pf *PortForward) {
	err := s.ctrlClient.Create(ctx, pf)
	if err != nil &&
		!apierrors.IsNotFound(err) &&
		!apierrors.IsConflict(err) &&
		!apierrors.IsAlreadyExists(err) {
		st.Dispatch(store.NewErrorAction(fmt.Errorf(
			"creating PortForward on apiserver: %v", err)))
		return
	}
	st.Dispatch(NewPortForwardCreateAction(pf))
}

// Extract the port-forward specs from the manifest.
//
// If any of them have ContainerPort = 0, populate them with the documented
// ports on the pod. If there's no default documented ports for the pod,
// populate it with the local port.
func populatePortForwards(m model.Manifest, pod v1alpha1.Pod) []model.PortForward {
	cPorts := store.AllPodContainerPorts(pod)
	fwds := m.K8sTarget().PortForwards
	forwards := make([]model.PortForward, 0, len(fwds))
	for _, forward := range fwds {
		if forward.ContainerPort == 0 && len(cPorts) > 0 {
			forward.ContainerPort = int(cPorts[0])
			for _, cPort := range cPorts {
				if int(forward.LocalPort) == int(cPort) {
					forward.ContainerPort = int(cPort)
					break
				}
			}
		}
		if forward.ContainerPort == 0 {
			forward.ContainerPort = forward.LocalPort
		}
		forwards = append(forwards, forward)
	}
	return forwards
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
