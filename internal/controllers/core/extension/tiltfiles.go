package extension

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	errorutil "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

var (
	apiGVStr = v1alpha1.SchemeGroupVersion.String()
	apiKind  = "Extension"
	apiType  = metav1.TypeMeta{Kind: apiKind, APIVersion: apiGVStr}
)

// Reconcile the Tiltfile owned by this Extension.
//
// TODO(nick): This is almost exactly the same as the code for managing PortForwards
// from KubernetesDiscovery. I feel like we're missing some abstraction here.
func (r *Reconciler) manageOwnedTiltfile(ctx context.Context, nn types.NamespacedName, owner *v1alpha1.Extension) error {
	var childList v1alpha1.TiltfileList
	err := indexer.ListOwnedBy(ctx, r.ctrlClient, &childList, nn, apiType)
	if err != nil {
		return fmt.Errorf("failed to fetch managed Tiltfile objects for Extension %s: %v",
			owner.Name, err)
	}

	child, err := r.toDesiredTiltfile(owner)
	if err != nil {
		return fmt.Errorf("creating tiltfile: %v", err)
	}

	// Delete all the Tiltfiles that don't match this one.
	errs := []error{}
	foundDesired := false
	for _, existingChild := range childList.Items {
		matches := child != nil && existingChild.Name == child.Name
		if matches {
			foundDesired = true

			if apicmp.DeepEqual(child.Spec, existingChild.Spec) {
				continue
			}

			updated := existingChild.DeepCopy()
			updated.Spec = child.Spec
			err := r.ctrlClient.Update(ctx, updated)
			if err != nil && !apierrors.IsNotFound(err) {
				errs = append(errs, fmt.Errorf("updating tiltfile %s: %v", existingChild.Name, err))
			}
			continue
		}

		// If this does not match the desired child, this child needs to be garbage collected.
		deletedChild := existingChild.DeepCopy()
		err := r.ctrlClient.Delete(ctx, deletedChild)
		if err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("deleting tiltfile %s: %v", existingChild.Name, err))
		}
	}

	if !foundDesired && child != nil {
		err := r.ctrlClient.Create(ctx, child)
		if err != nil && !apierrors.IsAlreadyExists(err) {
			errs = append(errs, fmt.Errorf("creating tiltfile %s: %v", child.Name, err))
		}
	}

	return errorutil.NewAggregate(errs)
}

// Construct the desired tiltfile. May be nil.
func (r *Reconciler) toDesiredTiltfile(owner *v1alpha1.Extension) (*v1alpha1.Tiltfile, error) {
	if owner == nil {
		return nil, nil
	}

	path := owner.Status.Path
	if path == "" {
		return nil, nil
	}

	// Each extensions resources get their own group.
	// We prefix it with 'extension' so that all extensions get put together.
	// TODO(nick): Let the user choose the label when they register the extension.
	label := apis.SanitizeLabel(fmt.Sprintf("extension.%s", owner.Name))
	child := &v1alpha1.Tiltfile{
		ObjectMeta: metav1.ObjectMeta{
			Name: owner.Name,
			Annotations: map[string]string{
				v1alpha1.AnnotationManifest: owner.Name,
			},
		},
		Spec: v1alpha1.TiltfileSpec{
			Path: path,
			Labels: map[string]string{
				label: label,
			},
			Args: owner.Spec.Args,
		},
	}
	err := controllerutil.SetControllerReference(owner, child, r.ctrlClient.Scheme())
	if err != nil {
		return nil, err
	}
	return child, nil
}
