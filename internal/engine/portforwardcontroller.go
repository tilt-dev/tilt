package engine

import (
	"context"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"k8s.io/api/core/v1"
)

type PortForwardController struct {
	kClient k8s.Client

	activeForwards map[k8s.PodID]portForwardEntry
}

func NewPortForwardController(kClient k8s.Client) *PortForwardController {
	return &PortForwardController{
		kClient:        kClient,
		activeForwards: make(map[k8s.PodID]portForwardEntry),
	}
}

// Figure out the diff between what's in the data store and
// what port-forwarding is currently active.
func (m *PortForwardController) diff(ctx context.Context, st store.RStore) (toStart []portForwardEntry, toShutdown []portForwardEntry) {
	state := st.RLockState()
	defer st.RUnlockState()

	statePods := make(map[k8s.PodID]bool, len(state.ManifestStates))

	// Find all the port-forwards that need to be created.
	for _, ms := range state.ManifestStates {
		pod := ms.MostRecentPod()
		podID := pod.PodID
		if podID == "" {
			continue
		}

		// Only do port-forwarding if the pod is running.
		if pod.Phase != v1.PodRunning && !pod.Deleting {
			continue
		}

		forwards := PopulatePortForwards(ms.Manifest, pod)
		if len(forwards) == 0 {
			continue
		}

		statePods[podID] = true

		_, isActive := m.activeForwards[podID]
		if isActive {
			continue
		}

		ctx, cancel := context.WithCancel(ctx)
		entry := portForwardEntry{
			podID:     podID,
			name:      ms.Manifest.Name,
			namespace: pod.Namespace,
			forwards:  forwards,
			ctx:       ctx,
			cancel:    cancel,
		}

		toStart = append(toStart, entry)
		m.activeForwards[podID] = entry
	}

	// Find all the port-forwards that aren't in the manifest anymore
	// and need to be shutdown.
	for key, value := range m.activeForwards {
		_, inState := statePods[key]
		if inState {
			continue
		}

		toShutdown = append(toShutdown, value)
		delete(m.activeForwards, key)
	}

	return toStart, toShutdown
}

func (m *PortForwardController) OnChange(ctx context.Context, st store.RStore) {
	toStart, toShutdown := m.diff(ctx, st)
	for _, entry := range toShutdown {
		entry.cancel()
	}

	for _, entry := range toStart {
		entry := entry
		ns := entry.namespace
		podID := entry.podID
		for _, forward := range entry.forwards {
			// TODO(nick): Handle the case where DockerForDesktop is handling
			// the port-forwarding natively already
			_, closer, err := m.kClient.ForwardPort(ctx, ns, podID, forward.LocalPort, forward.ContainerPort)
			if err != nil {
				logger.Get(ctx).Infof("Error port-forwarding %s: %v", entry.name, err)
				continue
			}

			go func() {
				<-entry.ctx.Done()
				closer()
			}()
		}
	}
}

var _ store.Subscriber = &PortForwardController{}

type portForwardEntry struct {
	name               model.ManifestName
	namespace          k8s.Namespace
	podID              k8s.PodID
	firstContainerPort int32
	forwards           []model.PortForward
	ctx                context.Context
	cancel             func()
}

// Extract the port-forward specs from the manifest. If any of them
// have ContainerPort = 0, populate them with the default port in the pod spec.
// Quietly drop forwards that we can't populate.
func PopulatePortForwards(m model.Manifest, pod store.Pod) []model.PortForward {
	cPorts := pod.ContainerPorts
	fwds := m.K8sInfo().PortForwards
	forwards := make([]model.PortForward, 0, len(fwds))
	for _, forward := range fwds {
		if forward.ContainerPort == 0 {
			if len(cPorts) == 0 {
				continue
			}

			forward.ContainerPort = int(cPorts[0])
		}
		forwards = append(forwards, forward)
	}
	return forwards
}
