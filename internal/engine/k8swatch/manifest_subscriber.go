package k8swatch

import (
	"context"
	"fmt"
	"strings"

	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/tilt-dev/tilt/internal/k8s"

	"k8s.io/apimachinery/pkg/api/equality"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type ManifestSubscriber struct {
	cfgNS k8s.Namespace
}

func NewManifestSubscriber(cfgNS k8s.Namespace) *ManifestSubscriber {
	return &ManifestSubscriber{
		cfgNS: cfgNS,
	}
}

func (m *ManifestSubscriber) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) {
	if summary.IsLogOnly() {
		return
	}

	state := st.RLockState()
	defer st.RUnlockState()

	claims := make(map[types.UID]types.NamespacedName)
	seen := make(map[types.NamespacedName]bool)
	for _, mt := range state.Targets() {
		key := KeyForManifest(mt.Manifest.Name)
		seen[key] = true
		kd, err := m.kubernetesDiscoveryFromManifest(ctx, key, mt, claims)
		if err != nil {
			// if the error is logged, it'll just trigger another store change and loop back here and
			// get logged over and over, so all errors are fatal; any errors returned by the generation
			// logic are indicative of a bug/regression, so this is fine
			st.Dispatch(store.NewErrorAction(fmt.Errorf(
				"failed to create KubernetesDiscovery spec for resource %q: %v",
				mt.Manifest.Name, err)))
		}

		existing := state.KubernetesDiscoveries[key]
		if kd != nil && existing == nil {
			st.Dispatch(NewKubernetesDiscoveryCreateAction(kd))
		} else if kd != nil && existing != nil {
			if !equality.Semantic.DeepEqual(existing.Spec, kd.Spec) {
				st.Dispatch(NewKubernetesDiscoveryUpdateAction(kd))
			}
		} else if kd == nil && existing != nil {
			// this manifest was modified such that it has no K8s entities to watch,
			// so just delete the entity
			st.Dispatch(NewKubernetesDiscoveryDeleteAction(key))
		}
	}

	for key := range state.KubernetesDiscoveries {
		if !seen[key] {
			// this manifest was deleted entirely
			st.Dispatch(NewKubernetesDiscoveryDeleteAction(key))
		}
	}
}

func KeyForManifest(mn model.ManifestName) types.NamespacedName {
	return types.NamespacedName{Name: apis.SanitizeName(mn.String())}
}

// kubernetesDiscoveryFromManifest creates a spec from a manifest.
//
// Because there is no graceful way to handle errors without triggering infinite loops in the store,
// any returned error should be considered fatal.
func (m *ManifestSubscriber) kubernetesDiscoveryFromManifest(ctx context.Context, key types.NamespacedName, mt *store.ManifestTarget, claims map[types.UID]types.NamespacedName) (*v1alpha1.KubernetesDiscovery, error) {
	if !mt.Manifest.IsK8s() {
		return nil, nil
	}
	kt := mt.Manifest.K8sTarget()

	krs := mt.State.K8sRuntimeState()
	if len(kt.ObjectRefs) == 0 {
		// there is nothing to discover
		return nil, nil
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

	// HACK(milas): until these are stored in apiserver, explicitly ensure they're valid
	// 	(any failure here is indicative of a logic bug in this method)
	if fieldErrs := kd.Validate(ctx); len(fieldErrs) != 0 {
		var sb strings.Builder
		for i, fieldErr := range fieldErrs {
			if i != 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fieldErr.Error())
		}
		return nil, fmt.Errorf("failed validation: %s", sb.String())
	}

	return kd, nil
}
