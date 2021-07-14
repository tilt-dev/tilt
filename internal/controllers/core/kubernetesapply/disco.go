package kubernetesapply

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// Each KubernetesApply object owns a KubernetesDiscovery object of the same name.
//
// After we reconcile a KubernetesApply, update the KubernetesDiscovery objects it owns.
//
// If the Apply has been deleted, any corresponding Disco objects should be deleted.
func (r *Reconciler) manageOwnedKubernetesDiscovery(ctx context.Context, nn types.NamespacedName, ka *v1alpha1.KubernetesApply) error {
	if !r.enableDiscoForTesting {
		// TODO(nick): remove this.
		return nil
	}

	if ka != nil && (ka.Status.Error != "" || ka.Status.ResultYAML == "") {
		// If the KubernetesApply is in an error state or hasn't deployed anything, don't
		// reconcile the discovery object. This prevents the reconcilers
		// from tearing down all the discovery infra on a transient deploy error.
		return nil
	}

	var existingKD v1alpha1.KubernetesDiscovery
	err := r.ctrlClient.Get(ctx, nn, &existingKD)
	isNotFound := apierrors.IsNotFound(err)
	if err != nil && !isNotFound {
		return fmt.Errorf("failed to fetch managed KubernetesDiscovery objects for KubernetesApply %s: %v",
			ka.Name, err)
	}

	kd, err := r.toDesiredKubernetesDiscovery(ka)
	if err != nil {
		return fmt.Errorf("generating kubernetesdiscovery: %v", err)
	}

	if isNotFound {
		if kd == nil {
			return nil // Nothing to do.
		}

		err := r.ctrlClient.Create(ctx, kd)
		if err != nil {
			return fmt.Errorf("creating kubernetesdiscovery: %v", err)
		}
		return nil
	}

	if kd == nil {
		err := r.ctrlClient.Delete(ctx, &existingKD)
		if err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("deleting kubernetesdiscovery: %v", err)
		}
		return nil
	}

	if !apicmp.DeepEqual(existingKD.Spec, kd.Spec) {
		existingKD.Spec = kd.Spec
		err = r.ctrlClient.Update(ctx, &existingKD)
		if err != nil {
			return fmt.Errorf("updating kubernetesdiscovery: %v", err)
		}
	}

	return nil
}

// Construct the desired KubernetesDiscovery
func (r *Reconciler) toDesiredKubernetesDiscovery(ka *v1alpha1.KubernetesApply) (*v1alpha1.KubernetesDiscovery, error) {
	if ka == nil {
		return nil, nil
	}

	watchRefs, err := r.toWatchRefs(ka)
	if err != nil {
		return nil, err
	}

	if len(watchRefs) == 0 {
		return nil, nil
	}

	kapp := ka.Spec
	var extraSelectors []metav1.LabelSelector
	if kapp.KubernetesDiscoveryTemplateSpec != nil {
		extraSelectors = kapp.KubernetesDiscoveryTemplateSpec.ExtraSelectors
	}

	kd := &v1alpha1.KubernetesDiscovery{
		ObjectMeta: metav1.ObjectMeta{
			Name: ka.Name,
			Annotations: map[string]string{
				v1alpha1.AnnotationManifest: ka.Annotations[v1alpha1.AnnotationManifest],
				v1alpha1.AnnotationSpanID:   ka.Annotations[v1alpha1.AnnotationSpanID],
			},
		},
		Spec: v1alpha1.KubernetesDiscoverySpec{
			Watches:                  watchRefs,
			ExtraSelectors:           extraSelectors,
			PodLogStreamTemplateSpec: kapp.PodLogStreamTemplateSpec.DeepCopy(),
			PortForwardTemplateSpec:  kapp.PortForwardTemplateSpec.DeepCopy(),
		},
	}

	err = controllerutil.SetControllerReference(ka, kd, r.ctrlClient.Scheme())
	if err != nil {
		return nil, err
	}
	return kd, nil
}

// Based on the deployed UIDs, create the list of resources to watch.
//
// TODO(nick): This currently does a lot of YAML parsing, just to get a few small
// metadata fields. We should be able to do better here if it becomes a problem, by either
// 1) optimizing the parsing, or
// 2) memoizing the Apply -> Discovery function
func (r *Reconciler) toWatchRefs(ka *v1alpha1.KubernetesApply) ([]v1alpha1.KubernetesWatchRef, error) {
	seenNamespaces := make(map[k8s.Namespace]bool)
	var result []v1alpha1.KubernetesWatchRef
	if ka.Status.ResultYAML != "" {
		deployed, err := k8s.ParseYAMLFromString(ka.Status.ResultYAML)
		if err != nil {
			return nil, err
		}
		deployedRefs := k8s.ToRefList(deployed)

		for _, ref := range deployedRefs {
			ns := k8s.Namespace(ref.Namespace)
			if ns == "" {
				// since this entity is actually deployed, don't fallback to cfgNS
				ns = k8s.DefaultNamespace
			}
			seenNamespaces[ns] = true
			result = append(result, v1alpha1.KubernetesWatchRef{
				UID:       string(ref.UID),
				Namespace: ns.String(),
				Name:      ref.Name,
			})
		}
	}

	entities, err := k8s.ParseYAMLFromString(ka.Spec.YAML)
	if err != nil {
		return nil, err
	}

	for _, e := range entities {
		ns := k8s.Namespace(e.Meta().GetNamespace())
		if ns == "" {
			ns = r.cfgNS
		}
		if ns == "" {
			ns = k8s.DefaultNamespace
		}
		if !seenNamespaces[ns] {
			seenNamespaces[ns] = true
			result = append(result, v1alpha1.KubernetesWatchRef{
				Namespace: ns.String(),
			})
		}
	}

	return result, nil
}
