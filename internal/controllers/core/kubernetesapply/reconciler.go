package kubernetesapply

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/docker/distribution/reference"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"github.com/tilt-dev/tilt/internal/build"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/apis/configmap"
	"github.com/tilt-dev/tilt/internal/controllers/apis/restarton"
	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/localexec"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/kubernetesapplys"
	"github.com/tilt-dev/tilt/internal/timecmp"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type deleteSpec struct {
	entities  []k8s.K8sEntity
	deleteCmd *v1alpha1.KubernetesApplyCmd
}

type Reconciler struct {
	st          store.RStore
	dkc         build.DockerKubeConnection
	kubeContext k8s.KubeContext
	k8sClient   k8s.Client
	cfgNS       k8s.Namespace
	ctrlClient  ctrlclient.Client
	indexer     *indexer.Indexer
	execer      localexec.Execer

	mu sync.Mutex

	// Protected by the mutex.
	results map[types.NamespacedName]*Result
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.KubernetesApply{}).
		Owns(&v1alpha1.KubernetesDiscovery{}).
		Watches(&source.Kind{Type: &v1alpha1.ImageMap{}},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue)).
		Watches(&source.Kind{Type: &v1alpha1.ConfigMap{}},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue))

	restarton.SetupController(b, r.indexer, func(obj ctrlclient.Object) (*v1alpha1.RestartOnSpec, *v1alpha1.StartOnSpec) {
		ka := obj.(*v1alpha1.KubernetesApply)
		return ka.Spec.RestartOn, nil
	})

	return b, nil
}

func NewReconciler(ctrlClient ctrlclient.Client, k8sClient k8s.Client, scheme *runtime.Scheme, dkc build.DockerKubeConnection, kubeContext k8s.KubeContext, st store.RStore, cfgNS k8s.Namespace, execer localexec.Execer) *Reconciler {
	return &Reconciler{
		ctrlClient:  ctrlClient,
		k8sClient:   k8sClient,
		indexer:     indexer.NewIndexer(scheme, indexKubernetesApply),
		execer:      execer,
		dkc:         dkc,
		kubeContext: kubeContext,
		st:          st,
		results:     make(map[types.NamespacedName]*Result),
		cfgNS:       cfgNS,
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
		err := r.bestEffortDeleteWithRelatedObjects(ctx, nn)
		if err != nil {
			return ctrl.Result{}, err
		}

		r.st.Dispatch(kubernetesapplys.NewKubernetesApplyDeleteAction(request.NamespacedName.Name))
		return ctrl.Result{}, nil
	}

	// Get configmap's disable status
	disableStatus, err := configmap.MaybeNewDisableStatus(ctx, r.ctrlClient, ka.Spec.DisableSource, ka.Status.DisableStatus)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Update kubernetesapply's disable status
	if disableStatus != ka.Status.DisableStatus {
		ka.Status.DisableStatus = disableStatus
		if err := r.ctrlClient.Status().Update(ctx, &ka); err != nil {
			return ctrl.Result{}, err
		}
	}

	// Delete kubernetesapply if it's disabled
	if disableStatus.Disabled {
		err := r.bestEffortDeleteWithRelatedObjects(ctx, nn)
		if err != nil {
			return ctrl.Result{}, err
		}

		r.st.Dispatch(kubernetesapplys.NewKubernetesApplyUpsertAction(&ka))
		return ctrl.Result{}, nil
	}

	// The apiserver is the source of truth, and will ensure the engine state is up to date.
	r.st.Dispatch(kubernetesapplys.NewKubernetesApplyUpsertAction(&ka))

	ctx = store.MustObjectLogHandler(ctx, r.st, &ka)
	err = r.manageOwnedKubernetesDiscovery(ctx, nn, &ka)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Fetch all the images needed to apply this YAML.
	imageMaps := make(map[types.NamespacedName]*v1alpha1.ImageMap)
	for _, name := range ka.Spec.ImageMaps {
		var im v1alpha1.ImageMap
		nn := types.NamespacedName{Name: name}
		err := r.ctrlClient.Get(ctx, nn, &im)
		if err != nil {
			if apierrors.IsNotFound(err) {
				// If the map isn't found, keep going and let shouldDeployOnReconcile
				// handle it.
				continue
			}
			return ctrl.Result{}, err
		}

		imageMaps[nn] = &im
	}

	restartObjs, err := restarton.FetchObjects(ctx, r.ctrlClient, ka.Spec.RestartOn, nil)
	if err != nil {
		return ctrl.Result{}, err
	}

	if !r.shouldDeployOnReconcile(request.NamespacedName, &ka, imageMaps, restartObjs) {
		// TODO(nick): Like with other reconcilers, there should always
		// be a reason why we're not deploying, and we should update the
		// Status field of KubernetesApply with that reason.
		return ctrl.Result{}, nil
	}

	// Update the apiserver with the result of this deploy.
	_, err = r.ForceApply(ctx, nn, ka.Spec, imageMaps)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// Determine if we should deploy the current YAML.
//
// Ensures:
// 1) We have enough info to deploy, and
// 2) Either we haven't deployed before,
//    or one of the inputs has changed since the last deploy.
func (r *Reconciler) shouldDeployOnReconcile(nn types.NamespacedName, ka *v1alpha1.KubernetesApply,
	imageMaps map[types.NamespacedName]*v1alpha1.ImageMap, restartObjs restarton.Objects) bool {
	if ka.Annotations[v1alpha1.AnnotationManagedBy] != "" {
		// Until resource dependencies are expressed in the API,
		// we can't use reconciliation to deploy KubernetesApply objects
		// managed by the buildcontrol engine.
		return false
	}

	for _, imageMapName := range ka.Spec.ImageMaps {
		_, ok := imageMaps[types.NamespacedName{Name: imageMapName}]
		if !ok {
			// We haven't built the images yet to deploy.
			return false
		}
	}

	r.mu.Lock()
	result, ok := r.results[nn]
	r.mu.Unlock()

	if !ok || result.Status.LastApplyTime.IsZero() {
		// We've never successfully deployed before, so deploy now.
		return true
	}

	if !apicmp.DeepEqual(ka.Spec, result.Spec) {
		// The YAML to deploy changed.
		return true
	}

	imageMapNames := ka.Spec.ImageMaps
	if len(imageMapNames) != len(result.ImageMapSpecs) ||
		len(imageMapNames) != len(result.ImageMapStatuses) {
		return true
	}

	for i, name := range ka.Spec.ImageMaps {
		im := imageMaps[types.NamespacedName{Name: name}]
		if !apicmp.DeepEqual(im.Spec, result.ImageMapSpecs[i]) {

			return true
		}
		if !apicmp.DeepEqual(im.Status, result.ImageMapStatuses[i]) {
			return true
		}
	}

	lastRestartTime, _ := restarton.LastRestartEvent(ka.Spec.RestartOn, restartObjs)
	if !timecmp.BeforeOrEqual(lastRestartTime, result.Status.LastApplyTime) {
		return true
	}

	return false
}

// Inject the images into the YAML and apply it to the cluster, unconditionally.
//
// Update the apiserver when finished.
//
// We expose this as a public method as a hack! Currently, in Tilt, BuildController
// handles dependencies between resources. The API server doesn't know about build
// dependencies yet. So Tiltfile-owned resources are applied manually, rather than
// going through the normal reconcile system.
func (r *Reconciler) ForceApply(
	ctx context.Context,
	nn types.NamespacedName,
	spec v1alpha1.KubernetesApplySpec,
	imageMaps map[types.NamespacedName]*v1alpha1.ImageMap) (v1alpha1.KubernetesApplyStatus, error) {

	var ka v1alpha1.KubernetesApply
	err := r.ctrlClient.Get(ctx, nn, &ka)
	if err != nil {
		return v1alpha1.KubernetesApplyStatus{}, err // TODO (lizz): Will this empty status return affect anything consuming this data?
	}

	status, appliedObjects := r.forceApplyHelper(ctx, r.ctrlClient, spec, ka.Status, imageMaps)
	statusCopy := status.DeepCopy()
	result := Result{
		Spec:           spec,
		Status:         *statusCopy,
		AppliedObjects: newObjectRefSet(appliedObjects),
	}

	for _, imageMapName := range spec.ImageMaps {
		im, ok := imageMaps[types.NamespacedName{Name: imageMapName}]
		if !ok {
			// this should never happen, but if it does, just continue quietly.
			continue
		}

		result.ImageMapSpecs = append(result.ImageMapSpecs, im.Spec)
		result.ImageMapStatuses = append(result.ImageMapStatuses, im.Status)
	}

	ka.Status = status
	err = r.ctrlClient.Status().Update(ctx, &ka)
	if err != nil {
		return status, err
	}

	toDelete := r.updateResult(nn, &result)
	r.bestEffortDelete(ctx, toDelete)

	return status, nil
}

// A helper that applies the given specs to the cluster, but doesn't update the APIServer.
//
// Returns:
// - the new status to store in the apiserver
// - the parsed entities that we tried to apply
func (r *Reconciler) forceApplyHelper(
	ctx context.Context,
	ctrlClient ctrlclient.Client,
	spec v1alpha1.KubernetesApplySpec,
	prevStatus v1alpha1.KubernetesApplyStatus,
	imageMaps map[types.NamespacedName]*v1alpha1.ImageMap,
) (v1alpha1.KubernetesApplyStatus, []k8s.K8sEntity) {

	startTime := apis.NowMicro()
	status := v1alpha1.KubernetesApplyStatus{
		LastApplyStartTime: startTime,
	}

	errorStatus := func(err error) v1alpha1.KubernetesApplyStatus {
		status.LastApplyTime = apis.NowMicro()
		status.Error = err.Error()
		return status
	}

	inputHash, err := ComputeInputHash(spec, imageMaps)
	if err != nil {
		return errorStatus(err), nil
	}

	var deployed []k8s.K8sEntity
	if spec.YAML != "" {
		deployed, err = r.runYAMLDeploy(ctx, spec, imageMaps)
		if err != nil {
			return errorStatus(err), nil
		}
	} else {
		deployed, err = r.runCmdDeploy(ctx, spec)
		if err != nil {
			return errorStatus(err), nil
		}
	}

	status.LastApplyTime = apis.NowMicro()
	status.AppliedInputHash = inputHash
	for _, d := range deployed {
		d.Clean()
	}

	resultYAML, err := k8s.SerializeSpecYAML(deployed)
	if err != nil {
		return errorStatus(err), deployed
	}

	status.ResultYAML = resultYAML
	return status, deployed
}

func (r *Reconciler) runYAMLDeploy(ctx context.Context, spec v1alpha1.KubernetesApplySpec, imageMaps map[types.NamespacedName]*v1alpha1.ImageMap) ([]k8s.K8sEntity, error) {
	// Create API objects.
	newK8sEntities, err := r.createEntitiesToDeploy(ctx, imageMaps, spec)
	if err != nil {
		return newK8sEntities, err
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
		return nil, err
	}

	return deployed, nil
}

func (r *Reconciler) runCmdDeploy(ctx context.Context, spec v1alpha1.KubernetesApplySpec) ([]k8s.K8sEntity, error) {
	timeout := spec.Timeout.Duration
	if timeout == 0 {
		timeout = v1alpha1.KubernetesApplyTimeoutDefault
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var stdoutBuf bytes.Buffer
	runIO := localexec.RunIO{
		Stdout: &stdoutBuf,
		Stderr: logger.Get(ctx).Writer(logger.InfoLvl),
	}

	exitCode, err := r.execer.Run(ctx, toModelCmd(*spec.DeployCmd), runIO)
	if err != nil {
		return nil, fmt.Errorf("apply command failed: %v", err)
	} else if exitCode != 0 {
		return nil, fmt.Errorf("apply command exited with status %d\nstdout:\n%s\n", exitCode, stdoutBuf.String())
	}

	// don't pass the bytes.Buffer directly to the YAML parser or it'll consume it and we can't print it out on failure
	stdout := stdoutBuf.Bytes()
	entities, err := k8s.ParseYAML(bytes.NewReader(stdout))
	if err != nil {
		return nil, fmt.Errorf("apply command returned malformed YAML: %v\nstdout:\n%s\n", err, string(stdout))
	}

	return entities, nil
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

// We keep track of all the objects it's managing in the cluster, and
// garbage-collect them when it no longer needs to manage them.
//
// A best-practices reconciler would store this info with the objects themselves
// (in the cluster), similar to how Helm does it.
//
// But for now, we store this as in-memory state, because it's cheaper to implement
// that way.
//
// Returns: objects to garbage-collect.
func (r *Reconciler) updateResult(nn types.NamespacedName, result *Result) deleteSpec {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing := r.results[nn]
	if result == nil {
		delete(r.results, nn)
	} else {
		r.results[nn] = result
	}

	if result != nil && result.Status.Error != "" {
		// do not attempt to delete any objects if the apply failed
		// N.B. if the result is nil, that means the object was deleted, so objects WILL be deleted
		return deleteSpec{}
	}

	if existing == nil {
		// there is no prior state, so we have nothing to GC
		return deleteSpec{}
	}

	if result == nil && existing.Spec.DeleteCmd != nil {
		// the object was deleted (so result is nil) and we have a custom delete cmd, so use that
		// and skip diffing managed entities entirely
		return deleteSpec{deleteCmd: existing.Spec.DeleteCmd}
	}

	// Go through all the results, and check to see which objects
	// we're not managing anymore.
	// TODO(milas): in the case that the KA object was deleted, should we respect `tilt.dev/down-policy`?
	toDeleteMap := existing.AppliedObjects.clone()
	for _, result := range r.results {
		for objRef := range result.AppliedObjects {
			delete(toDeleteMap, objRef)
		}
	}

	toDelete := make([]k8s.K8sEntity, 0, len(toDeleteMap))
	for _, e := range toDeleteMap {
		toDelete = append(toDelete, e)
	}
	return deleteSpec{entities: toDelete}
}

// A helper that deletes all kubernetesapply objects and the
// related kubernetesdiscovery objects it owns
func (r *Reconciler) bestEffortDeleteWithRelatedObjects(
	ctx context.Context,
	nn types.NamespacedName,
) error {
	toDelete := r.updateResult(nn, nil)
	r.bestEffortDelete(ctx, toDelete)

	err := r.manageOwnedKubernetesDiscovery(ctx, nn, nil)
	if err != nil {
		return err
	}

	return nil
}

func (r *Reconciler) bestEffortDelete(ctx context.Context, toDelete deleteSpec) {
	if len(toDelete.entities) == 0 && toDelete.deleteCmd == nil {
		return
	}

	l := logger.Get(ctx)
	l.Infof("Garbage collecting Kubernetes resources:")

	if len(toDelete.entities) != 0 {
		// Use a min component count of 2 for computing names,
		// so that the resource type appears
		displayNames := k8s.UniqueNames(toDelete.entities, 2)
		for _, displayName := range displayNames {
			l.Infof("→ %s", displayName)
		}

		err := r.k8sClient.Delete(ctx, toDelete.entities)
		if err != nil {
			l.Errorf("Error garbage collecting Kubernetes resources: %v", err)
		}
	}

	if toDelete.deleteCmd != nil {
		deleteCmd := toModelCmd(*toDelete.deleteCmd)
		l.Infof("Running cmd: %s", deleteCmd.String())

		out := l.Writer(logger.InfoLvl)
		runIO := localexec.RunIO{Stdout: out, Stderr: out}
		exitCode, err := r.execer.Run(ctx, deleteCmd, runIO)
		if err == nil && exitCode != 0 {
			err = fmt.Errorf("exit status %d", exitCode)
		}
		if err != nil {
			l.Errorf("Error garbage collecting Kubernetes resources: %v", err)
		}
	}
}

var imGVK = v1alpha1.SchemeGroupVersion.WithKind("ImageMap")

// indexKubernetesApply returns keys for all the objects we need to watch based on the spec.
func indexKubernetesApply(obj client.Object) []indexer.Key {
	ka := obj.(*v1alpha1.KubernetesApply)
	result := []indexer.Key{}
	for _, name := range ka.Spec.ImageMaps {
		result = append(result, indexer.Key{
			Name: types.NamespacedName{Name: name},
			GVK:  imGVK,
		})
	}

	if ka.Spec.DisableSource != nil {
		cm := ka.Spec.DisableSource.ConfigMap
		if cm != nil {
			cmGVK := v1alpha1.SchemeGroupVersion.WithKind("ConfigMap")
			result = append(result, indexer.Key{
				Name: types.NamespacedName{Name: cm.Name},
				GVK:  cmGVK,
			})
		}
	}
	return result
}

// Keeps track of the state we currently know about.
type Result struct {
	Spec             v1alpha1.KubernetesApplySpec
	ImageMapSpecs    []v1alpha1.ImageMapSpec
	ImageMapStatuses []v1alpha1.ImageMapStatus

	AppliedObjects objectRefSet
	Status         v1alpha1.KubernetesApplyStatus
}

type objectRef struct {
	Name       string
	Namespace  string
	APIVersion string
	Kind       string
}

type objectRefSet map[objectRef]k8s.K8sEntity

func newObjectRefSet(entities []k8s.K8sEntity) objectRefSet {
	r := make(objectRefSet, len(entities))
	for _, e := range entities {
		ref := e.ToObjectReference()
		oRef := objectRef{
			Name:       ref.Name,
			Namespace:  ref.Namespace,
			APIVersion: ref.APIVersion,
			Kind:       ref.Kind,
		}
		r[oRef] = e
	}
	return r
}

func (s objectRefSet) clone() objectRefSet {
	result := make(objectRefSet, len(s))
	for k, v := range s {
		result[k] = v
	}
	return result
}

func toModelCmd(cmd v1alpha1.KubernetesApplyCmd) model.Cmd {
	return model.Cmd{
		Argv: cmd.Args,
		Dir:  cmd.Dir,
		Env:  cmd.Env,
	}
}
