package dockercomposeservice

import (
	"context"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// Each DockerComposeService object owns a DockerComposeLogStream object of the same name.
//
// NOTE(nick): I think this might be the wrong API. Currently we model DockerCompose as two objects:
//
//  1. A service object with a Spec that tells us how to create the service and
//     a Status that contains the container state
//  2. A log object with a Spec that tells us how to watch logs.
//
// I think the better way to model this would be to more clearly separate "spinning up a service"
// from "watching a service", so we'd actually have:
//
//  1. A service object with a Spec that tells us how to create the service and
//     a Status that tells us whether the creation succeeded.
//  2. A monitor object with a status that contains both the container state and the log state.
//
// But moving to this system will be easier once everything is in the API server.
func (r *Reconciler) manageOwnedLogStream(ctx context.Context, nn types.NamespacedName, obj *v1alpha1.DockerComposeService) (reconcile.Result, error) {
	if obj != nil && (obj.Status.ApplyError != "" || obj.Status.ContainerID == "") {
		isDisabled := obj.Status.DisableStatus != nil &&
			obj.Status.DisableStatus.State == v1alpha1.DisableStateDisabled
		if !isDisabled {
			// If the DockerCompose is in an error state or hasn't deployed anything,
			// don't reconcile the log object.
			return reconcile.Result{}, nil
		}
	}

	var existing v1alpha1.DockerComposeLogStream
	err := r.ctrlClient.Get(ctx, nn, &existing)
	isNotFound := apierrors.IsNotFound(err)
	if err != nil && !isNotFound {
		return reconcile.Result{},
			fmt.Errorf("failed to fetch managed DockerComposeLogStream objects for DockerComposeService %s: %v",
				nn.Name, err)
	}

	desired, err := r.toDesiredLogStream(obj)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("generating dockercomposelogstream: %v", err)
	}

	if isNotFound {
		if desired == nil {
			return reconcile.Result{}, nil // Nothing to do.
		}

		err := r.ctrlClient.Create(ctx, desired)
		if err != nil {
			if apierrors.IsAlreadyExists(err) {
				return reconcile.Result{RequeueAfter: time.Second}, nil
			}
			return reconcile.Result{}, fmt.Errorf("creating dockercomposelogstream: %v", err)
		}
		return reconcile.Result{}, nil
	}

	if desired == nil {
		err := r.ctrlClient.Delete(ctx, &existing)
		if err != nil && !apierrors.IsNotFound(err) {
			return reconcile.Result{}, fmt.Errorf("deleting dockercomposelogstream: %v", err)
		}
		return reconcile.Result{}, nil
	}

	if !apicmp.DeepEqual(existing.Spec, desired.Spec) ||
		!apicmp.DeepEqual(existing.ObjectMeta.Annotations, desired.ObjectMeta.Annotations) {
		existing = *existing.DeepCopy()
		existing.ObjectMeta.Annotations = desired.ObjectMeta.Annotations
		existing.Spec = desired.Spec
		err = r.ctrlClient.Update(ctx, &existing)
		if err != nil {
			if apierrors.IsConflict(err) {
				return reconcile.Result{RequeueAfter: time.Second}, nil
			}
			return reconcile.Result{}, fmt.Errorf("updating dockercomposelogstream: %v", err)
		}
	}

	return reconcile.Result{}, nil
}

// Construct the desired DockerComposeLogStream
func (r *Reconciler) toDesiredLogStream(obj *v1alpha1.DockerComposeService) (*v1alpha1.DockerComposeLogStream, error) {
	if obj == nil {
		return nil, nil
	}

	if obj.Status.DisableStatus != nil && obj.Status.DisableStatus.State == v1alpha1.DisableStateDisabled {
		return nil, nil
	}

	desired := &v1alpha1.DockerComposeLogStream{
		ObjectMeta: metav1.ObjectMeta{
			Name: obj.Name,
			Annotations: map[string]string{
				v1alpha1.AnnotationManifest: obj.Annotations[v1alpha1.AnnotationManifest],
				v1alpha1.AnnotationSpanID:   obj.Annotations[v1alpha1.AnnotationSpanID],
			},
		},
		Spec: v1alpha1.DockerComposeLogStreamSpec{
			Service: obj.Spec.Service,
			Project: obj.Spec.Project,
		},
	}

	err := controllerutil.SetControllerReference(obj, desired, r.ctrlClient.Scheme())
	if err != nil {
		return nil, err
	}
	return desired, nil
}
