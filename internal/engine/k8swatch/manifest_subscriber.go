package k8swatch

import (
	"context"
	"fmt"
	"strings"

	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/tilt-dev/tilt/internal/k8s"

	"k8s.io/apimachinery/pkg/api/equality"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
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

	seen := make(map[types.NamespacedName]bool)
	for _, mt := range state.Targets() {
		key := keyForManifest(mt.Manifest.Name)
		seen[key] = true
		kd := m.kubernetesDiscoveryFromManifest(ctx, key, mt)
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

func keyForManifest(mn model.ManifestName) types.NamespacedName {
	return types.NamespacedName{Name: apis.SanitizeName(mn.String())}
}

func labelsFromSelector(selector labels.Selector) ([]v1alpha1.LabelValue, error) {
	var out []v1alpha1.LabelValue
	requirements, _ := selector.Requirements()
	for _, r := range requirements {
		if r.Operator() != selection.Equals {
			return nil, fmt.Errorf("label %q has unsupported operator: %q", r.Key(), r.Operator())
		}
		values := r.Values().List()
		if len(values) == 0 {
			continue
		}
		if len(values) != 1 {
			return nil, fmt.Errorf("label %q has more than one value: %v", r.Key(), r.Values())
		}
		out = append(out, v1alpha1.LabelValue{Label: r.Key(), Value: values[0]})
	}
	return out, nil
}

func (m *ManifestSubscriber) kubernetesDiscoveryFromManifest(ctx context.Context, key types.NamespacedName, mt *store.ManifestTarget) *v1alpha1.KubernetesDiscovery {
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

	var labelSets [][]v1alpha1.LabelValue
	if len(seenNamespaces) > 0 {
		for i := range kt.ExtraPodSelectors {
			l, err := labelsFromSelector(kt.ExtraPodSelectors[i])
			if err != nil {
				logger.Get(ctx).Debugf("Error processing extra pod selectors for %q: %v", kt.Name, err)
				return nil
			}
			labelSets = append(labelSets, l)
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
			ExtraSelectors: labelSets,
		},
	}

	if fieldErrs := kd.Validate(ctx); len(fieldErrs) != 0 {
		var sb strings.Builder
		for i, fieldErr := range fieldErrs {
			if i != 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fieldErr.Error())
		}
		logger.Get(ctx).Debugf("Manifest %q produced invalid spec: %s",
			mt.Manifest.Name, sb.String())
		return nil
	}

	return kd
}
