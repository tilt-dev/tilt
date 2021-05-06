package runtimelog

import (
	"context"
	"fmt"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store/k8sconv"

	"github.com/tilt-dev/tilt/pkg/apis"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

const IstioInitContainerName = container.Name("istio-init")
const IstioSidecarContainerName = container.Name("istio-proxy")

// Translates EngineState into PodLogWatch API objects
type PodLogManager struct {
	client ctrlclient.Client
}

func NewPodLogManager(client ctrlclient.Client) *PodLogManager {
	return &PodLogManager{client: client}
}

// Diff the current watches against the state store of what
// we're supposed to be watching, returning the changes
// we need to make.
func (m *PodLogManager) diff(ctx context.Context, st store.RStore) (setup []*PodLogStream, teardown []*PodLogStream) {
	state := st.RLockState()
	defer st.RUnlockState()

	current := map[types.NamespacedName]*PodLogStream{}
	for _, pls := range state.PodLogStreams {
		current[types.NamespacedName{Name: pls.Spec.Pod, Namespace: pls.Spec.Namespace}] = pls
	}
	seen := map[types.NamespacedName]bool{}

	for _, mt := range state.Targets() {
		man := mt.Manifest

		// Skip logs that don't come from tiltfile-generated manifests
		// (in particular, the local metrics stack).
		if man.Source != model.ManifestSourceTiltfile {
			continue
		}

		ms := mt.State
		runtime := ms.K8sRuntimeState()
		for _, pod := range runtime.PodList() {
			if pod.Name == "" {
				continue
			}

			nn := types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}
			sinceTime := apis.NewTime(state.TiltStartTime)
			spec := PodLogStreamSpec{
				Pod:       pod.Name,
				Namespace: pod.Namespace,
				SinceTime: &sinceTime,
				IgnoreContainers: []string{
					string(IstioInitContainerName),
					string(IstioSidecarContainerName),
				},
			}
			obj := &PodLogStream{
				ObjectMeta: ObjectMeta{
					Name: fmt.Sprintf("%s-%s", pod.Namespace, pod.Name),
					Annotations: map[string]string{
						v1alpha1.AnnotationManifest: string(man.Name),
						v1alpha1.AnnotationSpanID:   string(k8sconv.SpanIDForPod(man.Name, k8s.PodID(pod.Name))),
					},
				},
				Spec: spec,
			}

			if _, ok := current[nn]; !ok {
				setup = append(setup, obj)
			}
			seen[nn] = true
		}
	}

	for key, pls := range current {
		if !seen[key] {
			teardown = append(teardown, pls)
		}
	}
	return setup, teardown
}

func (m *PodLogManager) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) {
	if len(summary.KubernetesDiscoveries.Changes) == 0 {
		return
	}

	setup, teardown := m.diff(ctx, st)
	for _, pls := range teardown {
		m.deletePls(ctx, st, pls)
	}

	for _, pls := range setup {
		m.createPls(ctx, st, pls)
	}
}

// Delete the PodLogStream API object. Should be idempotent.
func (m *PodLogManager) deletePls(ctx context.Context, st store.RStore, pls *PodLogStream) {
	err := m.client.Delete(ctx, pls)
	if err != nil &&
		!apierrors.IsNotFound(err) {
		st.Dispatch(store.NewErrorAction(fmt.Errorf("deleting PodLogStream from apiserver: %v", err)))
		return
	}
	st.Dispatch(PodLogStreamDeleteAction{Name: pls.Name})
}

// Create a PodLogStream API object, if necessary. Should be idempotent.
func (m *PodLogManager) createPls(ctx context.Context, st store.RStore, pls *PodLogStream) {
	err := m.client.Create(ctx, pls)
	if err != nil &&
		!apierrors.IsNotFound(err) &&
		!apierrors.IsConflict(err) &&
		!apierrors.IsAlreadyExists(err) {
		st.Dispatch(store.NewErrorAction(fmt.Errorf("creating PodLogStream on apiserver: %v", err)))
		return
	}
	st.Dispatch(PodLogStreamCreateAction{PodLogStream: pls})
}

var _ store.Subscriber = &PodLogManager{}
