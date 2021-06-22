package kubernetesapply

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/builder"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
)

type Reconciler struct {
	st          store.RStore
	dkc         build.DockerKubeConnection
	kubeContext k8s.KubeContext
	k8sClient   k8s.Client
	ctrlClient  ctrlclient.Client
	indexer     *indexer.Indexer
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.KubernetesApply{}).
		Watches(&source.Kind{Type: &v1alpha1.ImageMap{}},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue))

	return b, nil
}

func NewReconciler(ctrlClient ctrlclient.Client, k8sClient k8s.Client, scheme *runtime.Scheme, dkc build.DockerKubeConnection, kubeContext k8s.KubeContext, st store.RStore) *Reconciler {
	return &Reconciler{
		ctrlClient:  ctrlClient,
		k8sClient:   k8sClient,
		indexer:     indexer.NewIndexer(scheme, indexImageMap),
		dkc:         dkc,
		kubeContext: kubeContext,
		st:          st,
	}
}

// Reconcile manages namespace watches for the modified KubernetesApply object.
func (r *Reconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	nn := request.NamespacedName

	var ka v1alpha1.KubernetesApply
	err := r.ctrlClient.Get(ctx, nn, &ka)
	r.indexer.OnReconcile(nn, &ka)
	if err != nil && !apierrors.IsNotFound(err) {
		return ctrl.Result{}, err
	}

	if apierrors.IsNotFound(err) || !ka.ObjectMeta.DeletionTimestamp.IsZero() {
		// delete
		return ctrl.Result{}, nil
	}

	// TODO: apply
	ctx = store.MustObjectLogHandler(ctx, r.st, &ka)
	_ = ctx

	return ctrl.Result{}, err
}

// Inject the images into the YAML and apply it to the cluster, unconditionally.
//
// We expose this as a public method as a hack! Currently, in Tilt, BuildController
// handles dependencies between resources. The API server doesn't know about build
// dependencies yet. So Tiltfile-owned resources are applied manually, rather than
// going through the normal reconcile system.
func (r *Reconciler) ForceApply(
	ctx context.Context,
	spec v1alpha1.KubernetesApplySpec,
	imageMaps map[types.NamespacedName]*v1alpha1.ImageMap) v1alpha1.KubernetesApplyStatus {

	status := v1alpha1.KubernetesApplyStatus{}

	errorStatus := func(err error) v1alpha1.KubernetesApplyStatus {
		status.LastApplyTime = apis.NowMicro()
		status.Error = err.Error()
		return status
	}

	inputHash, err := ComputeInputHash(spec, imageMaps)
	if err != nil {
		return errorStatus(err)
	}

	// Create API objects.
	newK8sEntities, err := r.createEntitiesToDeploy(ctx, imageMaps, spec)
	if err != nil {
		return errorStatus(err)
	}

	ctx = r.indentLogger(ctx)
	l := logger.Get(ctx)

	l.Infof("Applying via kubectl:")

	// Use a min component count of 2 for computing names,
	// so that the resource type appears
	displayNames := k8s.UniqueNames(newK8sEntities, 2)
	for _, displayName := range displayNames {
		l.Infof("→ %s", displayName)
	}

	timeout := spec.Timeout.Duration
	if timeout == 0 {
		timeout = v1alpha1.KubernetesApplyTimeoutDefault
	}

	deployed, err := r.k8sClient.Upsert(ctx, newK8sEntities, timeout)
	if err != nil {
		return errorStatus(err)
	}

	status.AppliedInputHash = inputHash
	for _, d := range deployed {
		d.Clean()
	}

	resultYAML, err := k8s.SerializeSpecYAML(deployed)
	if err != nil {
		return errorStatus(err)
	}

	status.ResultYAML = resultYAML
	return status
}

func (r *Reconciler) indentLogger(ctx context.Context) context.Context {
	l := logger.Get(ctx)
	newL := logger.NewPrefixedLogger(logger.Blue(l).Sprint("     "), l)
	return logger.WithLogger(ctx, newL)
}

func (r *Reconciler) createEntitiesToDeploy(ctx context.Context,
	imageMaps map[types.NamespacedName]*v1alpha1.ImageMap,
	spec v1alpha1.KubernetesApplySpec) ([]k8s.K8sEntity, error) {
	newK8sEntities := []k8s.K8sEntity{}

	entities, err := k8s.ParseYAMLFromString(spec.YAML)
	if err != nil {
		return nil, err
	}

	locators, err := k8s.ParseImageLocators(spec.ImageLocators)
	if err != nil {
		return nil, err
	}

	imageMapNames := spec.ImageMaps
	injectedImageMaps := map[string]bool{}
	for _, e := range entities {
		e, err = k8s.InjectLabels(e, []model.LabelPair{
			k8s.TiltManagedByLabel(),
		})
		if err != nil {
			return nil, errors.Wrap(err, "deploy")
		}

		// If we're redeploying these workloads in response to image
		// changes, we make sure image pull policy isn't set to "Always".
		// Frequent applies don't work well with this setting, and makes things
		// slower. See discussion:
		// https://github.com/tilt-dev/tilt/issues/3209
		if len(imageMaps) > 0 {
			e, err = k8s.InjectImagePullPolicy(e, v1.PullIfNotPresent)
			if err != nil {
				return nil, err
			}
		}

		if len(imageMaps) > 0 {
			// StatefulSet pods should be managed in parallel when we're doing iterative
			// development. See discussion:
			// https://github.com/tilt-dev/tilt/issues/1962
			// https://github.com/tilt-dev/tilt/issues/3906
			e = k8s.InjectParallelPodManagementPolicy(e)
		}

		// When working with a local k8s cluster, we set the pull policy to Never,
		// to ensure that k8s fails hard if the image is missing from docker.
		policy := v1.PullIfNotPresent
		if r.dkc.WillBuildToKubeContext(r.kubeContext) {
			policy = v1.PullNever
		}

		for _, imageMapName := range imageMapNames {
			imageMap := imageMaps[types.NamespacedName{Name: imageMapName}]
			imageMapSpec := imageMap.Spec
			selector, err := container.SelectorFromImageMap(imageMapSpec)
			if err != nil {
				return nil, err
			}
			matchInEnvVars := imageMapSpec.MatchInEnvVars

			if imageMap.Status.Image == "" {
				return nil, fmt.Errorf("internal error: missing image status")
			}

			ref, err := reference.ParseNamed(imageMap.Status.Image)
			if err != nil {
				return nil, fmt.Errorf("parsing image map status: %v", err)
			}

			var replaced bool
			e, replaced, err = k8s.InjectImageDigest(e, selector, ref, locators, matchInEnvVars, policy)
			if err != nil {
				return nil, err
			}
			if replaced {
				injectedImageMaps[imageMapName] = true

				if imageMapSpec.OverrideCommand != nil || imageMapSpec.OverrideArgs != nil {
					e, err = k8s.InjectCommandAndArgs(e, ref, imageMapSpec.OverrideCommand, imageMapSpec.OverrideArgs)
					if err != nil {
						return nil, err
					}
				}
			}
		}

		// This needs to be after all the other injections, to ensure the hash includes the Tilt-generated
		// image tag, etc
		e, err := k8s.InjectPodTemplateSpecHashes(e)
		if err != nil {
			return nil, errors.Wrap(err, "injecting pod template hash")
		}

		newK8sEntities = append(newK8sEntities, e)
	}

	for _, name := range imageMapNames {
		if !injectedImageMaps[name] {
			return nil, fmt.Errorf("Docker image missing from yaml: %s", name)
		}
	}

	return newK8sEntities, nil
}

var imGVK = v1alpha1.SchemeGroupVersion.WithKind("ImageMap")

// Find all the objects we need to watch based on the Cmd model.
func indexImageMap(obj client.Object) []indexer.Key {
	ka := obj.(*v1alpha1.KubernetesApply)
	result := []indexer.Key{}

	for _, name := range ka.Spec.ImageMaps {
		result = append(result, indexer.Key{
			Name: types.NamespacedName{Name: name},
			GVK:  imGVK,
		})
	}
	return result
}
