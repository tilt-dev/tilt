package k8swatch

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/tilt-dev/tilt/internal/k8s"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type ManifestSubscriber struct {
	cfgNS      k8s.Namespace
	client     ctrlclient.Client
	lastUpdate map[types.NamespacedName]*v1alpha1.KubernetesDiscoverySpec
}

func NewManifestSubscriber(cfgNS k8s.Namespace, client ctrlclient.Client) *ManifestSubscriber {
	return &ManifestSubscriber{
		cfgNS:      cfgNS,
		client:     client,
		lastUpdate: make(map[types.NamespacedName]*v1alpha1.KubernetesDiscoverySpec),
	}
}

// OnChange creates KubernetesDiscovery objects from the engine manifests' K8s targets.
//
// Because this runs extremely frequently and cannot rely on change summary to filter its work, it keeps
// copies of the latest versions it successfully persisted to the server in lastUpdate so that it can avoid
// unnecessary API calls.
//
// Currently, any unexpected API errors are fatal.
func (m *ManifestSubscriber) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) error {
	if summary.IsLogOnly() {
		return nil
	}

	current := m.makeSpecsFromEngineState(ctx, st)
	for key, kd := range current {
		existing := m.lastUpdate[key]
		if existing == nil {
			if err := m.createKubernetesDiscovery(ctx, st, key, &kd); err != nil {
				st.Dispatch(store.NewErrorAction(err))
				return nil
			}
		} else if !equality.Semantic.DeepEqual(existing, &kd.Spec) {
			err := m.updateKubernetesDiscovery(ctx, st, key, func(toUpdate *v1alpha1.KubernetesDiscovery) {
				toUpdate.Spec = kd.Spec
			})
			if err != nil {
				st.Dispatch(store.NewErrorAction(err))
				return nil
			}
		}
	}

	for key := range m.lastUpdate {
		if _, ok := current[key]; !ok {
			// this manifest was deleted or changed such that it has nothing to watch
			if err := m.deleteKubernetesDiscovery(ctx, st, key); err != nil {
				st.Dispatch(store.NewErrorAction(err))
				return nil
			}
		}
	}

	return nil
}

func (m *ManifestSubscriber) getKubernetesDiscovery(ctx context.Context, key types.NamespacedName) (*v1alpha1.KubernetesDiscovery, error) {
	var kd v1alpha1.KubernetesDiscovery
	if err := m.client.Get(ctx, key, &kd); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get KubernetesDiscovery %q: %v", key, err)
	}
	return &kd, nil
}

func (m *ManifestSubscriber) createKubernetesDiscovery(ctx context.Context, st store.RStore, key types.NamespacedName, kd *v1alpha1.KubernetesDiscovery) error {
	err := m.client.Create(ctx, kd)
	if err != nil {
		return fmt.Errorf("failed to create KubernetesDiscovery %q: %v", key, err)
	}
	m.lastUpdate[key] = kd.Spec.DeepCopy()
	st.Dispatch(NewKubernetesDiscoveryCreateAction(kd))
	return nil
}

func (m *ManifestSubscriber) updateKubernetesDiscovery(ctx context.Context, st store.RStore, key types.NamespacedName,
	updateFunc func(toUpdate *v1alpha1.KubernetesDiscovery)) error {

	kd, err := m.getKubernetesDiscovery(ctx, key)
	if err != nil {
		return err
	}
	if kd == nil {
		return nil
	}
	updateFunc(kd)

	err = m.client.Update(ctx, kd)
	if err == nil {
		m.lastUpdate[key] = kd.Spec.DeepCopy()
		st.Dispatch(NewKubernetesDiscoveryUpdateAction(kd))
	} else if !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
		return fmt.Errorf("failed to update KubernetesDiscovery %q: %v", key, err)
	}
	return nil
}

func (m *ManifestSubscriber) deleteKubernetesDiscovery(ctx context.Context, st store.RStore, key types.NamespacedName) error {
	kd, err := m.getKubernetesDiscovery(ctx, key)
	if err != nil {
		return err
	}
	if kd == nil {
		// already deleted
		return nil
	}

	err = m.client.Delete(ctx, kd)
	if ctrlclient.IgnoreNotFound(err) == nil {
		delete(m.lastUpdate, key)
		st.Dispatch(NewKubernetesDiscoveryDeleteAction(key))
	} else if !apierrors.IsNotFound(err) {
		return fmt.Errorf("failed to delete KubernetesDiscovery %q: %v", key, err)
	}
	return nil
}

func (m *ManifestSubscriber) makeSpecsFromEngineState(ctx context.Context, st store.RStore) map[types.NamespacedName]v1alpha1.KubernetesDiscovery {
	state := st.RLockState()
	defer st.RUnlockState()

	results := make(map[types.NamespacedName]v1alpha1.KubernetesDiscovery)
	claims := make(map[types.UID]types.NamespacedName)
	for _, mt := range state.Targets() {
		key := KeyForManifest(mt.Manifest.Name)
		kd := m.kubernetesDiscoveryFromManifest(ctx, key, mt, claims)
		if kd != nil {
			results[key] = *kd
		}
	}

	return results
}

func KeyForManifest(mn model.ManifestName) types.NamespacedName {
	return types.NamespacedName{Name: apis.SanitizeName(mn.String())}
}

// kubernetesDiscoveryFromManifest creates a spec from a manifest.
//
// Because there is no graceful way to handle errors without triggering infinite loops in the store,
// any returned error should be considered fatal.
func (m *ManifestSubscriber) kubernetesDiscoveryFromManifest(_ context.Context, key types.NamespacedName, mt *store.ManifestTarget, claims map[types.UID]types.NamespacedName) *v1alpha1.KubernetesDiscovery {
	if !mt.Manifest.IsK8s() {
		return nil
	}
	kt := mt.Manifest.K8sTarget()

	krs := mt.State.K8sRuntimeState()
	if len(kt.ObjectRefs) == 0 {
		// there is nothing to discover
		return nil
	}

	seenNamespaces := make(map[k8s.Namespace]bool)
	var watchRefs []v1alpha1.KubernetesWatchRef
	for _, e := range krs.DeployedEntities {
		if _, claimed := claims[e.UID]; claimed {
			// it's possible for multiple manifests to reference the same entity; however, duplicate reporting
			// of the same K8s resources can cause confusing + odd behavior throughout other parts of the engine,
			// so we only allow the first manifest to "claim" it so that the others won't receive events for it
			// (note that manifest iteration order is deterministic, which ensures it doesn't flip-flop)
			continue
		}
		claims[e.UID] = key

		ns := k8s.Namespace(e.Namespace)
		if ns == "" {
			// since this entity is actually deployed, don't fallback to cfgNS
			ns = k8s.DefaultNamespace
		}
		seenNamespaces[ns] = true
		watchRefs = append(watchRefs, v1alpha1.KubernetesWatchRef{
			UID:       string(e.UID),
			Namespace: ns.String(),
			Name:      e.Name,
		})
	}

	for i := range kt.ObjectRefs {
		ns := k8s.Namespace(kt.ObjectRefs[i].Namespace)
		if ns == "" {
			ns = m.cfgNS
		}
		if ns == "" {
			ns = k8s.DefaultNamespace
		}
		if !seenNamespaces[ns] {
			seenNamespaces[ns] = true
			watchRefs = append(watchRefs, v1alpha1.KubernetesWatchRef{
				Namespace: ns.String(),
			})
		}
	}

	var extraSelectors []metav1.LabelSelector
	if len(seenNamespaces) > 0 {
		for i := range kt.ExtraPodSelectors {
			extraSelectors = append(extraSelectors, *metav1.SetAsLabelSelector(kt.ExtraPodSelectors[i]))
		}
	}

	kd := &v1alpha1.KubernetesDiscovery{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name,
			Namespace: key.Namespace,
			Annotations: map[string]string{
				v1alpha1.AnnotationManifest: mt.Manifest.Name.String(),
				v1alpha1.AnnotationTargetID: mt.State.TargetID().String(),
			},
		},
		Spec: v1alpha1.KubernetesDiscoverySpec{
			Watches:        watchRefs,
			ExtraSelectors: extraSelectors,
		},
	}

	return kd
}
