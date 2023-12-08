package kubernetesapply

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/pkg/errors"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/controllers/apicmp"
	"github.com/tilt-dev/tilt/internal/controllers/apis/configmap"
	"github.com/tilt-dev/tilt/internal/controllers/apis/imagemap"
	"github.com/tilt-dev/tilt/internal/controllers/apis/trigger"
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
	cluster   *v1alpha1.Cluster
}

type Reconciler struct {
	st         store.RStore
	k8sClient  k8s.Client
	ctrlClient ctrlclient.Client
	indexer    *indexer.Indexer
	execer     localexec.Execer
	requeuer   *indexer.Requeuer

	mu sync.Mutex

	// Protected by the mutex.
	results map[types.NamespacedName]*Result
}

func (r *Reconciler) CreateBuilder(mgr ctrl.Manager) (*builder.Builder, error) {
	b := ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.KubernetesApply{}).
		Owns(&v1alpha1.KubernetesDiscovery{}).
		WatchesRawSource(r.requeuer, handler.Funcs{}).
		Watches(&v1alpha1.ImageMap{},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue)).
		Watches(&v1alpha1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue)).
		Watches(&v1alpha1.Cluster{},
			handler.EnqueueRequestsFromMapFunc(r.indexer.Enqueue))

	trigger.SetupControllerRestartOn(b, r.indexer, func(obj ctrlclient.Object) *v1alpha1.RestartOnSpec {
		return obj.(*v1alpha1.KubernetesApply).Spec.RestartOn
	})

	return b, nil
}

func NewReconciler(ctrlClient ctrlclient.Client, k8sClient k8s.Client, scheme *runtime.Scheme, st store.RStore, execer localexec.Execer) *Reconciler {
	return &Reconciler{
		ctrlClient: ctrlClient,
		k8sClient:  k8sClient,
		indexer:    indexer.NewIndexer(scheme, indexKubernetesApply),
		execer:     execer,
		st:         st,
		results:    make(map[types.NamespacedName]*Result),
		requeuer:   indexer.NewRequeuer(),
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
		result, err := r.manageOwnedKubernetesDiscovery(ctx, nn, nil)
		if err != nil {
			return ctrl.Result{}, err
		}

		r.recordDelete(nn)
		toDelete := r.garbageCollect(nn, true)
		r.bestEffortDelete(ctx, nn, toDelete, "garbage collecting Kubernetes objects")
		r.clearRecord(nn)

		r.st.Dispatch(kubernetesapplys.NewKubernetesApplyDeleteAction(request.NamespacedName.Name))
		return result, nil
	}

	// The apiserver is the source of truth, and will ensure the engine state is up to date.
	r.st.Dispatch(kubernetesapplys.NewKubernetesApplyUpsertAction(&ka))

	// Get configmap's disable status
	ctx = store.MustObjectLogHandler(ctx, r.st, &ka)
	disableStatus, err := configmap.MaybeNewDisableStatus(ctx, r.ctrlClient, ka.Spec.DisableSource, ka.Status.DisableStatus)
	if err != nil {
		return ctrl.Result{}, err
	}

	r.recordDisableStatus(nn, ka.Spec, *disableStatus)

	// Delete kubernetesapply if it's disabled
	isDisabling := false
	gcReason := "garbage collecting Kubernetes objects"
	if disableStatus.State == v1alpha1.DisableStateDisabled {
		gcReason = "deleting disabled Kubernetes objects"
		isDisabling = true
	} else {
		// Fetch all the objects needed to apply this YAML.
		var cluster v1alpha1.Cluster
		if ka.Spec.Cluster != "" {
			err := r.ctrlClient.Get(ctx, types.NamespacedName{Name: ka.Spec.Cluster}, &cluster)
			if client.IgnoreNotFound(err) != nil {
				return ctrl.Result{}, err
			}
		}

		imageMaps, err := imagemap.NamesToObjects(ctx, r.ctrlClient, ka.Spec.ImageMaps)
		if err != nil {
			return ctrl.Result{}, err
		}

		lastRestartEvent, _, _, err := trigger.LastRestartEvent(ctx, r.ctrlClient, ka.Spec.RestartOn)
		if err != nil {
			return ctrl.Result{}, err
		}

		// Apply to the cluster if necessary.
		//
		// TODO(nick): Like with other reconcilers, there should always
		// be a reason why we're not deploying, and we should update the
		// Status field of KubernetesApply with that reason.
		if r.shouldDeployOnReconcile(request.NamespacedName, &ka, &cluster, imageMaps, lastRestartEvent) {
			_ = r.forceApplyHelper(ctx, nn, ka.Spec, &cluster, imageMaps)
			gcReason = "garbage collecting removed Kubernetes objects"
		}
	}

	toDelete := r.garbageCollect(nn, isDisabling)
	r.bestEffortDelete(ctx, nn, toDelete, gcReason)

	newKA, err := r.maybeUpdateStatus(ctx, nn, &ka)
	if err != nil {
		return ctrl.Result{}, err
	}

	return r.manageOwnedKubernetesDiscovery(ctx, nn, newKA)
}

// Determine if we should deploy the current YAML.
//
// Ensures:
//  1. We have enough info to deploy, and
//  2. Either we haven't deployed before,
//     or one of the inputs has changed since the last deploy.
func (r *Reconciler) shouldDeployOnReconcile(
	nn types.NamespacedName,
	ka *v1alpha1.KubernetesApply,
	cluster *v1alpha1.Cluster,
	imageMaps map[types.NamespacedName]*v1alpha1.ImageMap,
	lastRestartEvent metav1.MicroTime,
) bool {
	if ka.Annotations[v1alpha1.AnnotationManagedBy] != "" {
		// Until resource dependencies are expressed in the API,
		// we can't use reconciliation to deploy KubernetesApply objects
		// managed by the buildcontrol engine.
		return false
	}

	if ka.Spec.Cluster != "" {
		isClusterOK := cluster != nil && cluster.Name != "" &&
			cluster.Status.Error == "" && cluster.Status.Connection != nil
		if !isClusterOK {
			// Wait for the cluster to start.
			return false
		}
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

	if timecmp.After(lastRestartEvent, result.Status.LastApplyTime) {
		return true
	}

	return false
}

// Inject the images into the YAML and apply it to the cluster, unconditionally.
//
// Does not update the API server, but does trigger a re-reconcile
// so that the reconciliation loop will handle it.
//
// We expose this as a public method as a hack! Currently, in Tilt, BuildController
// handles dependencies between resources. The API server doesn't know about build
// dependencies yet. So Tiltfile-owned resources are applied manually, rather than
// going through the normal reconcile system.
func (r *Reconciler) ForceApply(
	ctx context.Context,
	nn types.NamespacedName,
	spec v1alpha1.KubernetesApplySpec,
	cluster *v1alpha1.Cluster,
	imageMaps map[types.NamespacedName]*v1alpha1.ImageMap) v1alpha1.KubernetesApplyStatus {
	status := r.forceApplyHelper(ctx, nn, spec, cluster, imageMaps)
	r.requeuer.Add(nn)
	return status
}

// A helper that applies the given specs to the cluster,
// tracking the state of the deploy in the results map.
func (r *Reconciler) forceApplyHelper(
	ctx context.Context,
	nn types.NamespacedName,
	spec v1alpha1.KubernetesApplySpec,
	cluster *v1alpha1.Cluster,
	imageMaps map[types.NamespacedName]*v1alpha1.ImageMap,
) v1alpha1.KubernetesApplyStatus {

	startTime := apis.NowMicro()
	status := applyResult{
		LastApplyStartTime: startTime,
	}

	recordErrorStatus := func(err error) v1alpha1.KubernetesApplyStatus {
		status.LastApplyTime = apis.NowMicro()
		status.Error = err.Error()
		return r.recordApplyResult(nn, spec, cluster, imageMaps, status)
	}

	inputHash, err := ComputeInputHash(spec, imageMaps)
	if err != nil {
		return recordErrorStatus(err)
	}

	var deployed []k8s.K8sEntity
	deployCtx := r.indentLogger(ctx)
	if spec.YAML != "" {
		deployed, err = r.runYAMLDeploy(deployCtx, spec, imageMaps)
		if err != nil {
			return recordErrorStatus(err)
		}
	} else {
		deployed, err = r.runCmdDeploy(deployCtx, spec, cluster, imageMaps)
		if err != nil {
			return recordErrorStatus(err)
		}
	}

	status.LastApplyTime = apis.NowMicro()
	status.AppliedInputHash = inputHash
	for _, d := range deployed {
		d.Clean()
	}

	resultYAML, err := k8s.SerializeSpecYAML(deployed)
	if err != nil {
		return recordErrorStatus(err)
	}

	status.ResultYAML = resultYAML
	status.Objects = deployed
	return r.recordApplyResult(nn, spec, cluster, imageMaps, status)
}

func (r *Reconciler) printAppliedReport(ctx context.Context, msg string, deployed []k8s.K8sEntity) {
	l := logger.Get(ctx)
	l.Infof("%s", msg)

	// Use a min component count of 2 for computing names,
	// so that the resource type appears
	displayNames := k8s.UniqueNames(deployed, 2)
	for _, displayName := range displayNames {
		l.Infof("  → %s", displayName)
	}
}

func (r *Reconciler) runYAMLDeploy(ctx context.Context, spec v1alpha1.KubernetesApplySpec, imageMaps map[types.NamespacedName]*v1alpha1.ImageMap) ([]k8s.K8sEntity, error) {
	// Create API objects.
	newK8sEntities, err := r.createEntitiesToDeploy(ctx, imageMaps, spec)
	if err != nil {
		return newK8sEntities, err
	}

	logger.Get(ctx).Infof("Applying YAML to cluster")

	timeout := spec.Timeout.Duration
	if timeout == 0 {
		timeout = v1alpha1.KubernetesApplyTimeoutDefault
	}

	deployed, err := r.k8sClient.Upsert(ctx, newK8sEntities, timeout)
	if err != nil {
		r.printAppliedReport(ctx, "Tried to apply objects to cluster:", newK8sEntities)
		return nil, err
	}
	r.printAppliedReport(ctx, "Objects applied to cluster:", deployed)

	return deployed, nil
}

func (r *Reconciler) maybeInjectKubeconfig(cmd *model.Cmd, cluster *v1alpha1.Cluster) {
	if cluster == nil ||
		cluster.Status.Connection == nil ||
		cluster.Status.Connection.Kubernetes == nil {
		return
	}
	kubeconfig := cluster.Status.Connection.Kubernetes.ConfigPath
	if kubeconfig == "" {
		return
	}
	cmd.Env = append(cmd.Env, fmt.Sprintf("KUBECONFIG=%s", kubeconfig))
}

func (r *Reconciler) runCmdDeploy(ctx context.Context, spec v1alpha1.KubernetesApplySpec,
	cluster *v1alpha1.Cluster,
	imageMaps map[types.NamespacedName]*v1alpha1.ImageMap) ([]k8s.K8sEntity, error) {
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

	cmd := toModelCmd(*spec.ApplyCmd)
	err := imagemap.InjectIntoDeployEnv(&cmd, spec.ImageMaps, imageMaps)
	if err != nil {
		return nil, err
	}
	r.maybeInjectKubeconfig(&cmd, cluster)

	logger.Get(ctx).Infof("Running cmd: %s", cmd.String())
	exitCode, err := r.execer.Run(ctx, cmd, runIO)
	if err != nil {
		return nil, fmt.Errorf("apply command failed: %v", err)
	}

	if exitCode != 0 {
		var stdoutLog string
		if stdoutBuf.Len() != 0 {
			stdoutLog = fmt.Sprintf("\nstdout:\n%s\n", overflowEllipsis(stdoutBuf.String()))
		}
		if ctx.Err() != nil {
			// process returned a non-zero exit code (generally 137) because it was killed by us
			return nil, fmt.Errorf("apply command timed out after %s - see https://docs.tilt.dev/api.html#api.update_settings for how to increase%s", timeout.String(), stdoutLog)
		}
		return nil, fmt.Errorf("apply command exited with status %d%s", exitCode, stdoutLog)
	}

	// don't pass the bytes.Buffer directly to the YAML parser or it'll consume it and we can't print it out on failure
	stdout := stdoutBuf.Bytes()
	entities, err := k8s.ParseYAML(bytes.NewReader(stdout))
	if err != nil {
		return nil, fmt.Errorf("apply command returned malformed YAML: %v\nstdout:\n%s\n", err, overflowEllipsis(string(stdout)))
	}

	r.printAppliedReport(ctx, "Objects applied to cluster:", entities)

	return entities, nil
}

const maxOverflow = 500

// The stdout of a well-behaved apply function can be 100K+ (especially for CRDs)
func overflowEllipsis(str string) string {
	if len(str) > maxOverflow {
		return fmt.Sprintf("%s\n... [truncated by Tilt] ...\n%s", str[0:maxOverflow/2], str[len(str)-maxOverflow/2:])
	}
	return str
}

func (r *Reconciler) indentLogger(ctx context.Context) context.Context {
	l := logger.Get(ctx)
	newL := logger.NewPrefixedLogger(logger.Blue(l).Sprint("     "), l)
	return logger.WithLogger(ctx, newL)
}

type injectResult struct {
	meta     k8s.EntityMeta
	imageMap *v1alpha1.ImageMap
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

	var injectResults []injectResult
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

		// Set the pull policy to IfNotPresent, to ensure that
		// we get a locally built image instead of the remote one.
		policy := v1.PullIfNotPresent
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

			ref, err := container.ParseNamed(imageMap.Status.ImageFromCluster)
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
				injectResults = append(injectResults, injectResult{
					meta:     e,
					imageMap: imageMap,
				})

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

	l := logger.Get(ctx)
	if l.Level().ShouldDisplay(logger.DebugLvl) {
		if len(injectResults) != 0 {
			l.Debugf("Injecting images into Kubernetes YAML:")
			meta := make([]k8s.EntityMeta, len(injectResults))
			for i := range injectResults {
				meta[i] = injectResults[i].meta
			}
			names := k8s.UniqueNamesMeta(meta, 2)
			for i := range injectResults {
				l.Debugf(
					"  → %s: %s ⇒ %s",
					names[i],
					injectResults[i].imageMap.Spec.Selector,
					injectResults[i].imageMap.Status.Image,
				)
			}
		} else {
			l.Debugf("No images injected into Kubernetes YAML")
		}
	}

	return newK8sEntities, nil
}

type applyResult struct {
	ResultYAML         string
	Error              string
	LastApplyTime      metav1.MicroTime
	LastApplyStartTime metav1.MicroTime
	AppliedInputHash   string
	Objects            []k8s.K8sEntity
}

// conditionsFromApply extracts any conditions based on the result.
//
// Currently, this is only used as part of special handling for Jobs, which
// might have already completed successfully in the past.
func conditionsFromApply(result applyResult) []metav1.Condition {
	if result.Error != "" || len(result.Objects) == 0 {
		return nil
	}

	for _, e := range result.Objects {
		job, ok := e.Obj.(*batchv1.Job)
		if !ok {
			continue
		}
		for _, cond := range job.Status.Conditions {
			if cond.Type == batchv1.JobComplete && cond.Status == v1.ConditionTrue {
				return []metav1.Condition{
					{
						Type:   v1alpha1.ApplyConditionJobComplete,
						Status: metav1.ConditionTrue,
					},
				}
			}
		}
	}
	return nil
}

// Create a result object if necessary. Caller must hold the mutex.
func (r *Reconciler) ensureResultExists(nn types.NamespacedName) *Result {
	existing, hasExisting := r.results[nn]
	if hasExisting {
		return existing
	}

	result := &Result{
		DanglingObjects: objectRefSet{},
	}
	r.results[nn] = result
	return result
}

// Record the results of a deploy to the local Result map.
func (r *Reconciler) recordApplyResult(
	nn types.NamespacedName,
	spec v1alpha1.KubernetesApplySpec,
	cluster *v1alpha1.Cluster,
	imageMaps map[types.NamespacedName]*v1alpha1.ImageMap,
	applyResult applyResult) v1alpha1.KubernetesApplyStatus {

	r.mu.Lock()
	defer r.mu.Unlock()

	result := r.ensureResultExists(nn)

	// Copy over status information from `forceApplyHelper`
	// so other existing status information isn't overwritten
	updatedStatus := result.Status.DeepCopy()
	updatedStatus.ResultYAML = applyResult.ResultYAML
	updatedStatus.Error = applyResult.Error
	updatedStatus.LastApplyStartTime = applyResult.LastApplyStartTime
	updatedStatus.LastApplyTime = applyResult.LastApplyTime
	updatedStatus.AppliedInputHash = applyResult.AppliedInputHash
	updatedStatus.Conditions = conditionsFromApply(applyResult)

	result.Cluster = cluster
	result.Spec = spec
	result.Status = *updatedStatus
	if spec.ApplyCmd != nil {
		result.CmdApplied = true
	}
	result.SetAppliedObjects(newObjectRefSet(applyResult.Objects))

	result.ImageMapSpecs = nil
	result.ImageMapStatuses = nil
	for _, imageMapName := range spec.ImageMaps {
		im, ok := imageMaps[types.NamespacedName{Name: imageMapName}]
		if !ok {
			// this should never happen, but if it does, just continue quietly.
			continue
		}

		result.ImageMapSpecs = append(result.ImageMapSpecs, im.Spec)
		result.ImageMapStatuses = append(result.ImageMapStatuses, im.Status)
	}

	return result.Status
}

// Record that the apply has been disabled.
func (r *Reconciler) recordDisableStatus(
	nn types.NamespacedName,
	spec v1alpha1.KubernetesApplySpec,
	disableStatus v1alpha1.DisableStatus) {

	r.mu.Lock()
	defer r.mu.Unlock()

	result := r.ensureResultExists(nn)
	if apicmp.DeepEqual(result.Status.DisableStatus, &disableStatus) {
		return
	}

	isDisabled := disableStatus.State == v1alpha1.DisableStateDisabled

	update := result.Status.DeepCopy()
	update.DisableStatus = &disableStatus
	result.Status = *update

	if isDisabled {
		result.SetAppliedObjects(nil)
	}
}

// Queue its applied objects for deletion.
func (r *Reconciler) recordDelete(nn types.NamespacedName) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result := r.ensureResultExists(nn)
	result.Status = v1alpha1.KubernetesApplyStatus{}
	result.SetAppliedObjects(nil)
}

// Record that the delete command was run.
func (r *Reconciler) recordDeleteCmdRun(nn types.NamespacedName) {
	r.mu.Lock()
	defer r.mu.Unlock()

	result, isExisting := r.results[nn]
	if isExisting {
		result.CmdApplied = false
	}
}

// Delete all state for a KubernetesApply, things have been cleaned up.
func (r *Reconciler) clearRecord(nn types.NamespacedName) {
	r.mu.Lock()
	defer r.mu.Unlock()

	delete(r.results, nn)
}

// Perform garbage collection for a particular KubernetesApply object.
//
// isDeleting: indicates whether this is a full delete or just
// a cleanup of dangling objects.
//
// For custom deploy commands, we run the delete cmd.
//
// For YAML deploys, this is more complex:
//
// There are typically 4 ways objects get marked "dangling".
// 1) Their owner A has been deleted.
// 2) Their owner A has been disabled.
// 3) They've been moved from owner A to owner B.
// 4) Owner A has been re-applied with different arguments.
//
// Because the reconciler handles one owner at a time,
// cases (1) and (3) are basically indistinguishable, and can
// lead to race conditions if we're not careful (e.g., owner A's GC
// deletes objects deployed by B).
//
// TODO(milas): in the case that the KA object was deleted, should we respect `tilt.dev/down-policy`?
func (r *Reconciler) garbageCollect(nn types.NamespacedName, isDeleting bool) deleteSpec {
	r.mu.Lock()
	defer r.mu.Unlock()

	result, isExisting := r.results[nn]
	if !isExisting {
		return deleteSpec{}
	}

	if !isDeleting && result.Status.Error != "" {
		// do not attempt to delete any objects if the apply failed
		// N.B. if the result is nil, that means the object was deleted, so objects WILL be deleted
		return deleteSpec{}
	}

	if result.Spec.DeleteCmd != nil {
		if !isDeleting || !result.CmdApplied {
			// If there's a custom apply + delete command, GC only happens if
			// the KubernetesApply object is being deleted (or disabled) and
			// the apply command was actually executed (by Tilt).
			return deleteSpec{}
		}

		// the object was deleted (so result is nil) and we have a custom delete cmd, so use that
		// and skip diffing managed entities entirely
		//
		// We assume that the delete cmd deletes all dangling objects.
		for k := range result.DanglingObjects {
			delete(result.DanglingObjects, k)
		}
		result.clearApplyStatus()
		return deleteSpec{
			deleteCmd: result.Spec.DeleteCmd,
			cluster:   result.Cluster,
		}
	}

	// Reconcile the dangling objects against applied objects, ensuring that we're
	// not deleting an object that was moved to another resource.
	for _, result := range r.results {
		for objRef := range result.AppliedObjects {
			delete(result.DanglingObjects, objRef)
		}
	}

	toDelete := make([]k8s.K8sEntity, 0, len(result.DanglingObjects))
	for k, v := range result.DanglingObjects {
		delete(result.DanglingObjects, k)
		toDelete = append(toDelete, v)
	}
	if isDeleting {
		result.clearApplyStatus()
	}
	return deleteSpec{
		entities: toDelete,
		cluster:  result.Cluster,
	}
}

// A helper that deletes all Kubernetes objects, even if they haven't been applied yet.
//
// Namespaces are not deleted by default. Similar to `tilt down`, deleting namespaces
// is likely to be more destructive than most users want from this operation.
func (r *Reconciler) ForceDelete(ctx context.Context, nn types.NamespacedName,
	spec v1alpha1.KubernetesApplySpec,
	cluster *v1alpha1.Cluster,
	reason string) error {

	toDelete := deleteSpec{cluster: cluster}
	if spec.YAML != "" {
		entities, err := k8s.ParseYAMLFromString(spec.YAML)
		if err != nil {
			return fmt.Errorf("force delete: %v", err)
		}

		entities, _, err = k8s.Filter(entities, func(e k8s.K8sEntity) (b bool, err error) {
			return e.GVK() != schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}, nil
		})
		if err != nil {
			return err
		}

		toDelete.entities = k8s.ReverseSortedEntities(entities)
	} else if spec.DeleteCmd != nil {
		toDelete.deleteCmd = spec.DeleteCmd
	}

	r.recordDelete(nn)
	r.bestEffortDelete(ctx, nn, toDelete, reason)
	r.requeuer.Add(nn)
	return nil
}

// Update the status if necessary.
func (r *Reconciler) maybeUpdateStatus(ctx context.Context, nn types.NamespacedName, obj *v1alpha1.KubernetesApply) (*v1alpha1.KubernetesApply, error) {
	newStatus := v1alpha1.KubernetesApplyStatus{}
	existing, ok := r.results[nn]
	if ok {
		newStatus = existing.Status
	}

	if apicmp.DeepEqual(obj.Status, newStatus) {
		return obj, nil
	}

	oldError := obj.Status.Error
	newError := newStatus.Error
	update := obj.DeepCopy()
	update.Status = *(newStatus.DeepCopy())

	err := r.ctrlClient.Status().Update(ctx, update)
	if err != nil {
		return nil, err
	}

	// Print new errors on objects that aren't managed by the buildcontroller.
	if newError != "" && oldError != newError && update.Annotations[v1alpha1.AnnotationManagedBy] == "" {
		logger.Get(ctx).Errorf("kubernetesapply %s: %s", obj.Name, newError)
	}
	return update, nil
}

func (r *Reconciler) bestEffortDelete(ctx context.Context, nn types.NamespacedName, toDelete deleteSpec, reason string) {
	if len(toDelete.entities) == 0 && toDelete.deleteCmd == nil {
		return
	}

	l := logger.Get(ctx)
	l.Infof("Beginning %s", reason)

	if len(toDelete.entities) != 0 {
		err := r.k8sClient.Delete(ctx, toDelete.entities, 0)
		if err != nil {
			l.Errorf("Error %s: %v", reason, err)
		}
	}

	if toDelete.deleteCmd != nil {
		deleteCmd := toModelCmd(*toDelete.deleteCmd)
		r.maybeInjectKubeconfig(&deleteCmd, toDelete.cluster)
		if err := localexec.OneShotToLogger(ctx, r.execer, deleteCmd); err != nil {
			l.Errorf("Error %s: %v", reason, err)
		}
		r.recordDeleteCmdRun(nn)
	}
}

var imGVK = v1alpha1.SchemeGroupVersion.WithKind("ImageMap")
var clusterGVK = v1alpha1.SchemeGroupVersion.WithKind("Cluster")

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
	if ka.Spec.Cluster != "" {
		result = append(result, indexer.Key{
			Name: types.NamespacedName{Name: ka.Spec.Cluster},
			GVK:  clusterGVK,
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
	Cluster          *v1alpha1.Cluster
	ImageMapSpecs    []v1alpha1.ImageMapSpec
	ImageMapStatuses []v1alpha1.ImageMapStatus

	CmdApplied      bool
	AppliedObjects  objectRefSet
	DanglingObjects objectRefSet
	Status          v1alpha1.KubernetesApplyStatus
}

// Set the status of applied objects to empty,
// as if this had never been applied.
func (r *Result) clearApplyStatus() {
	if r.Status.LastApplyTime.IsZero() && r.Status.Error == "" {
		return
	}

	update := r.Status.DeepCopy()
	update.LastApplyTime = metav1.MicroTime{}
	update.LastApplyStartTime = metav1.MicroTime{}
	update.Error = ""
	update.ResultYAML = ""
	r.Status = *update
}

// Set a new collection of applied objects.
//
// Move all the currently applied objects to the dangling
// collection for garbage collection.
func (r *Result) SetAppliedObjects(set objectRefSet) {
	for k, v := range r.AppliedObjects {
		r.DanglingObjects[k] = v
	}
	r.AppliedObjects = set
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

func toModelCmd(cmd v1alpha1.KubernetesApplyCmd) model.Cmd {
	return model.Cmd{
		Argv: cmd.Args,
		Dir:  cmd.Dir,
		Env:  cmd.Env,
	}
}
