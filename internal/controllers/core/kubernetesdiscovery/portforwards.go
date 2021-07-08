package kubernetesdiscovery

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	errorutil "k8s.io/apimachinery/pkg/util/errors"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// Reconcile all the port forwards owned by this KD. The KD may be nil if it's being deleted.
func (r *Reconciler) manageOwnedPortForwards(ctx context.Context, nn types.NamespacedName, kd *v1alpha1.KubernetesDiscovery) error {
	var pfList v1alpha1.PortForwardList
	err := r.ctrlClient.List(ctx, &pfList, ctrlclient.InNamespace(nn.Namespace),
		ctrlclient.MatchingFields{ownerKey: nn.Name})
	if err != nil {
		return fmt.Errorf("failed to fetch managed PortForward objects for KubernetesDiscovery %s: %v",
			kd.Name, err)
	}

	pf, err := r.toDesiredPortForward(kd)
	if err != nil {
		return fmt.Errorf("creating portforward: %v", err)
	}

	// Delete all the port-forwards that don't match this one.
	errs := []error{}
	foundDesired := false
	for _, existingPF := range pfList.Items {
		matchesPF := pf != nil && existingPF.Name == pf.Name
		if matchesPF {
			foundDesired = true

			// If this PortForward is already in the APIServer, make sure it's up-to-date.
			if apicmp.DeepEqual(pf.Spec, existingPF) {
				continue
			}

			updatedPF := existingPF.DeepCopy()
			updatedPF.Spec = pf.Spec
			err := r.ctrlClient.Update(ctx, updatedPF)
			if err != nil && !apierrors.IsNotFound(err) {
				errs = append(errs, fmt.Errorf("updating portforward %s: %v", existingPF.Name, err))
			}
			continue
		}

		// If this does not match the desired PF, this PF needs to be garbage collected.
		deletedPF := existingPF.DeepCopy()
		err := r.ctrlClient.Delete(ctx, deletedPF)
		if err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("deleting portforward %s: %v", existingPF.Name, err))
		}
	}

	if !foundDesired && pf != nil {
		err := r.ctrlClient.Create(ctx, pf)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			errs = append(errs, fmt.Errorf("creating portforward %s: %v", pf.Name, err))
		}
	}

	return errorutil.NewAggregate(errs)
}

// Construct the desired port-forward. May be nil.
func (r *Reconciler) toDesiredPortForward(kd *v1alpha1.KubernetesDiscovery) (*v1alpha1.PortForward, error) {
	if kd == nil {
		return nil, nil
	}

	pfTemplate := kd.Spec.PortForwardTemplateSpec
	if pfTemplate == nil {
		return nil, nil
	}

	pod := pickBestPortForwardPod(kd)
	if pod == nil {
		return nil, nil
	}

	pf := &v1alpha1.PortForward{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s-%s", kd.Name, pod.Name),
			Annotations: map[string]string{
				v1alpha1.AnnotationManifest: kd.Annotations[v1alpha1.AnnotationManifest],
				v1alpha1.AnnotationSpanID:   kd.Annotations[v1alpha1.AnnotationSpanID],
			},
		},
		Spec: v1alpha1.PortForwardSpec{
			PodName:   pod.Name,
			Namespace: pod.Namespace,
			Forwards:  pfTemplate.Forwards,
		},
	}
	populateContainerPorts(pf, pod)
	err := controllerutil.SetControllerReference(kd, pf, r.ctrlClient.Scheme())
	if err != nil {
		return nil, err
	}
	return pf, nil
}

// If any of the port-forward specs have ContainerPort = 0, populate them with
// the documented ports on the pod. If there's no default documented ports for
// the pod, populate it with the local port.
//
// TODO(nick): This is old legacy behavior, and I'm not totally sure it even
// makes sense. I wonder if we should just insist that ContainerPort is populated.
func populateContainerPorts(pf *v1alpha1.PortForward, pod *v1alpha1.Pod) {
	cPorts := store.AllPodContainerPorts(*pod)
	for i, forward := range pf.Spec.Forwards {
		if forward.ContainerPort == 0 && len(cPorts) > 0 {
			forward.ContainerPort = int32(cPorts[0])
			for _, cPort := range cPorts {
				if int(forward.LocalPort) == int(cPort) {
					forward.ContainerPort = int32(cPort)
					break
				}
			}
		}
		if forward.ContainerPort == 0 {
			forward.ContainerPort = forward.LocalPort
		}
		pf.Spec.Forwards[i] = forward
	}
}

// We can only portforward to one pod at a time.
// So pick the "best" pod to portforward to.
// May be nil if there is no eligible pod.
func pickBestPortForwardPod(kd *v1alpha1.KubernetesDiscovery) *v1alpha1.Pod {
	var bestPod *v1alpha1.Pod
	for _, pod := range kd.Status.Pods {
		pod := pod
		if pod.Name == "" {
			continue
		}

		// Only do port-forwarding if the pod is running or deleting.
		isRunning := pod.Phase == string(v1.PodRunning)
		isDeleting := pod.Deleting
		if !isRunning && !isDeleting {
			continue
		}

		// This pod is eligible! Now compare it to the existing candidate (if there is one).
		if bestPod == nil || isBetterPortForwardPod(&pod, bestPod) {
			bestPod = &pod
		}
	}
	return bestPod
}

// Check if podA is better than podB for port-forwarding.
func isBetterPortForwardPod(podA, podB *v1alpha1.Pod) bool {
	// A non-deleting pod is always better than a deleting pod.
	if podB.Deleting && !podA.Deleting {
		return true
	} else if podA.Deleting && !podB.Deleting {
		return false
	}

	// Otherwise, a more recent pod is better.
	if podA.CreatedAt.After(podB.CreatedAt.Time) {
		return true
	} else if podB.CreatedAt.After(podA.CreatedAt.Time) {
		return false
	}

	// Use the name as a tie-breaker.
	return podA.Name > podB.Name
}
