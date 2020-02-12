package engine

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockerfile"
	"github.com/windmilleng/tilt/internal/engine/buildcontrol"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/synclet/sidecar"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
)

var _ BuildAndDeployer = &ImageBuildAndDeployer{}

type KINDLoader interface {
	LoadToKIND(ctx context.Context, ref reference.NamedTagged) error
}

type cmdKINDLoader struct {
	env         k8s.Env
	clusterName k8s.ClusterName
}

func (kl *cmdKINDLoader) LoadToKIND(ctx context.Context, ref reference.NamedTagged) error {
	// In Kind5, --name specifies the name of the cluster in the kubeconfig.
	// In Kind6, the -name parameter is prefixed with 'kind-' before being written to/read from the kubeconfig
	kindName := string(kl.clusterName)
	if kl.env == k8s.EnvKIND6 {
		kindName = strings.TrimPrefix(kindName, "kind-")
	}

	cmd := exec.CommandContext(ctx, "kind", "load", "docker-image", ref.String(), "--name", kindName)
	w := logger.NewMutexWriter(logger.Get(ctx).Writer(logger.InfoLvl))
	cmd.Stdout = w
	cmd.Stderr = w

	return cmd.Run()
}

func NewKINDLoader(env k8s.Env, clusterName k8s.ClusterName) KINDLoader {
	return &cmdKINDLoader{
		env:         env,
		clusterName: clusterName,
	}
}

type ImageBuildAndDeployer struct {
	ib               build.ImageBuilder
	icb              *imageAndCacheBuilder
	k8sClient        k8s.Client
	env              k8s.Env
	runtime          container.Runtime
	analytics        *analytics.TiltAnalytics
	injectSynclet    bool
	clock            build.Clock
	kl               KINDLoader
	syncletContainer sidecar.SyncletContainer
}

func NewImageBuildAndDeployer(
	b build.ImageBuilder,
	customBuilder build.CustomBuilder,
	k8sClient k8s.Client,
	env k8s.Env,
	analytics *analytics.TiltAnalytics,
	updMode buildcontrol.UpdateMode,
	c build.Clock,
	runtime container.Runtime,
	kl KINDLoader,
	syncletContainer sidecar.SyncletContainer,
) *ImageBuildAndDeployer {
	return &ImageBuildAndDeployer{
		ib:               b,
		icb:              NewImageAndCacheBuilder(b, customBuilder, updMode),
		k8sClient:        k8sClient,
		env:              env,
		analytics:        analytics,
		clock:            c,
		runtime:          runtime,
		kl:               kl,
		syncletContainer: syncletContainer,
	}
}

// Turn on synclet injection. Should be called before any builds.
func (ibd *ImageBuildAndDeployer) SetInjectSynclet(inject bool) {
	ibd.injectSynclet = inject
}

func (ibd *ImageBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, stateSet store.BuildStateSet) (resultSet store.BuildResultSet, err error) {
	iTargets, kTargets := extractImageAndK8sTargets(specs)
	if len(kTargets) != 1 {
		return store.BuildResultSet{}, buildcontrol.SilentRedirectToNextBuilderf("ImageBuildAndDeployer does not support these specs")
	}

	kTarget := kTargets[0]
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-ImageBuildAndDeployer-BuildAndDeploy")
	span.SetTag("target", kTarget.Name)
	defer span.Finish()

	startTime := time.Now()
	defer func() {
		ibd.analytics.Timer("build.image", time.Since(startTime), nil)
	}()

	q, err := buildcontrol.NewImageTargetQueue(ctx, iTargets, stateSet, ibd.ib.ImageExists)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	// each image target has two stages: one for build, and one for push
	numStages := q.CountDirty()*2 + 1

	ps := build.NewPipelineState(ctx, numStages, ibd.clock)
	defer func() { ps.End(ctx, err) }()

	var anyLiveUpdate bool

	iTargetMap := model.ImageTargetsByID(iTargets)
	err = q.RunBuilds(func(target model.TargetSpec, state store.BuildState, depResults []store.BuildResult) (store.BuildResult, error) {
		iTarget, ok := target.(model.ImageTarget)
		if !ok {
			return nil, fmt.Errorf("Not an image target: %T", target)
		}

		iTarget, err := injectImageDependencies(iTarget, iTargetMap, depResults)
		if err != nil {
			return nil, err
		}

		refs, err := ibd.icb.Build(ctx, iTarget, state, ps)
		if err != nil {
			return nil, err
		}

		err = ibd.push(ctx, refs.LocalRef, ps, iTarget, kTarget)
		if err != nil {
			return nil, err
		}

		anyLiveUpdate = anyLiveUpdate || !iTarget.LiveUpdateInfo().Empty()
		return store.NewImageBuildResult(iTarget.ID(), refs.LocalRef, refs.ClusterRef), nil
	})
	if err != nil {
		return store.BuildResultSet{}, buildcontrol.WrapDontFallBackError(err)
	}

	// (If we pass an empty list of refs here (as we will do if only deploying
	// yaml), we just don't inject any image refs into the yaml, nbd.
	brs, err := ibd.deploy(ctx, st, ps, iTargetMap, kTarget, q.Results(), anyLiveUpdate)
	return brs, buildcontrol.WrapDontFallBackError(err)
}

func (ibd *ImageBuildAndDeployer) push(ctx context.Context, ref reference.NamedTagged, ps *build.PipelineState, iTarget model.ImageTarget, kTarget model.K8sTarget) error {
	ps.StartPipelineStep(ctx, "Pushing %s", container.FamiliarString(ref))
	defer ps.EndPipelineStep(ctx)

	cbSkip := false
	if iTarget.IsCustomBuild() {
		cbSkip = iTarget.CustomBuildInfo().SkipsPush()
	}

	// We can also skip the push of the image if it isn't used
	// in any k8s resources! (e.g., it's consumed by another image).
	if ibd.canAlwaysSkipPush() || !isImageDeployedToK8s(iTarget, kTarget) || cbSkip {
		ps.Printf(ctx, "Skipping push")
		return nil
	}

	var err error
	if ibd.shouldUseKINDLoad(ctx, iTarget) {
		ps.Printf(ctx, "Loading image to KIND")
		err := ibd.kl.LoadToKIND(ps.AttachLogger(ctx), ref)
		if err != nil {
			return fmt.Errorf("Error loading image to KIND: %v", err)
		}
	} else {
		ps.Printf(ctx, "Pushing with Docker client")
		err = ibd.ib.PushImage(ps.AttachLogger(ctx), ref)
		if err != nil {
			return err
		}
	}

	return nil
}

func (ibd *ImageBuildAndDeployer) shouldUseKINDLoad(ctx context.Context, iTarg model.ImageTarget) bool {
	isKIND := ibd.env == k8s.EnvKIND5 || ibd.env == k8s.EnvKIND6
	if !isKIND {
		return false
	}

	// if we're using KIND and the image has a separate ref by which it's referred to
	// in the cluster, that implies that we have a local registry in place, and should
	// push to that instead of using KIND load.
	if iTarg.HasDistinctClusterRef() {
		return false
	}

	registry := ibd.k8sClient.LocalRegistry(ctx)
	if !registry.Empty() {
		return false
	}

	return true
}

// Returns: the entities deployed and the namespace of the pod with the given image name/tag.
func (ibd *ImageBuildAndDeployer) deploy(ctx context.Context, st store.RStore, ps *build.PipelineState,
	iTargetMap map[model.TargetID]model.ImageTarget, kTarget model.K8sTarget, results store.BuildResultSet, needsSynclet bool) (store.BuildResultSet, error) {
	ps.StartPipelineStep(ctx, "Deploying")
	defer ps.EndPipelineStep(ctx)

	ps.StartBuildStep(ctx, "Injecting images into Kubernetes YAML")

	newK8sEntities, err := ibd.createEntitiesToDeploy(ctx, iTargetMap, kTarget, results, needsSynclet)
	if err != nil {
		return nil, err
	}

	ctx = ibd.indentLogger(ctx)
	l := logger.Get(ctx)

	l.Infof("Applying via kubectl:")
	for _, displayName := range kTarget.DisplayNames {
		l.Infof("→ %s", displayName)
	}

	deployed, err := ibd.k8sClient.Upsert(ctx, newK8sEntities)
	if err != nil {
		return nil, err
	}

	// TODO(nick): Do something with this result
	uids := []types.UID{}
	podTemplateSpecHashes := []k8s.PodTemplateSpecHash{}
	for _, entity := range deployed {
		uid := entity.UID()
		if uid == "" {
			return nil, fmt.Errorf("Entity not deployed correctly: %v", entity)
		}
		uids = append(uids, entity.UID())
		hs, err := k8s.ReadPodTemplateSpecHashes(entity)
		if err != nil {
			return nil, errors.Wrap(err, "reading pod template spec hashes")
		}
		podTemplateSpecHashes = append(podTemplateSpecHashes, hs...)
	}
	results[kTarget.ID()] = store.NewK8sDeployResult(kTarget.ID(), uids, podTemplateSpecHashes, deployed)

	return results, nil
}

func (ibd *ImageBuildAndDeployer) indentLogger(ctx context.Context) context.Context {
	l := logger.Get(ctx)
	newL := logger.NewPrefixedLogger(logger.Blue(l).Sprint("     "), l)
	return logger.WithLogger(ctx, newL)
}

func (ibd *ImageBuildAndDeployer) createEntitiesToDeploy(ctx context.Context,
	iTargetMap map[model.TargetID]model.ImageTarget, k8sTarget model.K8sTarget,
	results store.BuildResultSet, needsSynclet bool) ([]k8s.K8sEntity, error) {
	newK8sEntities := []k8s.K8sEntity{}

	// TODO(nick): The parsed YAML should probably be a part of the model?
	// It doesn't make much sense to re-parse it and inject labels on every deploy.
	entities, err := k8s.ParseYAMLFromString(k8sTarget.YAML)
	if err != nil {
		return nil, err
	}

	depIDs := k8sTarget.DependencyIDs()
	injectedDepIDs := map[model.TargetID]bool{}
	for _, e := range entities {
		injectedSynclet := false
		e, err = k8s.InjectLabels(e, []model.LabelPair{
			k8s.TiltManagedByLabel(),
		})
		if err != nil {
			return nil, errors.Wrap(err, "deploy")
		}

		// For development, image pull policy should never be set to "Always",
		// even if it might make sense to use "Always" in prod. People who
		// set "Always" for development are shooting their own feet.
		e, err = k8s.InjectImagePullPolicy(e, v1.PullIfNotPresent)
		if err != nil {
			return nil, err
		}

		// StatefulSet pods should be managed in parallel. See:
		// https://github.com/windmilleng/tilt/issues/1962
		e = k8s.InjectParallelPodManagementPolicy(e)

		// When working with a local k8s cluster, we set the pull policy to Never,
		// to ensure that k8s fails hard if the image is missing from docker.
		policy := v1.PullIfNotPresent
		if ibd.canAlwaysSkipPush() {
			policy = v1.PullNever
		}

		for _, depID := range depIDs {
			ref := store.ClusterImageRefFromBuildResult(results[depID])
			if ref == nil {
				return nil, fmt.Errorf("Internal error: missing image build result for dependency ID: %s", depID)
			}

			iTarget := iTargetMap[depID]
			selector := iTarget.Refs.ConfigurationRef
			matchInEnvVars := iTarget.MatchInEnvVars

			var replaced bool
			e, replaced, err = k8s.InjectImageDigest(e, selector, ref, matchInEnvVars, policy)
			if err != nil {
				return nil, err
			}
			if replaced {
				injectedDepIDs[depID] = true

				if !iTarget.OverrideCmd.Empty() || iTarget.OverrideArgs.ShouldOverride {
					e, err = k8s.InjectCommandAndArgs(e, ref, iTarget.OverrideCmd, iTarget.OverrideArgs)
					if err != nil {
						return nil, err
					}
				}

				if ibd.injectSynclet && needsSynclet && !injectedSynclet {
					injectedRefSelector := container.NewRefSelector(ref).WithExactMatch()

					var sidecarInjected bool
					e, sidecarInjected, err = sidecar.InjectSyncletSidecar(e, injectedRefSelector, ibd.syncletContainer)
					if err != nil {
						return nil, err
					}
					if !sidecarInjected {
						return nil, fmt.Errorf("Could not inject synclet: %v", e)
					}
					injectedSynclet = true
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

	for _, depID := range depIDs {
		if !injectedDepIDs[depID] {
			return nil, fmt.Errorf("Docker image missing from yaml: %s", depID)
		}
	}

	return newK8sEntities, nil
}

// If we're using docker-for-desktop as our k8s backend,
// we don't need to push to the central registry.
// The k8s will use the image already available
// in the local docker daemon.
func (ibd *ImageBuildAndDeployer) canAlwaysSkipPush() bool {
	return ibd.env.UsesLocalDockerRegistry() && ibd.runtime == container.RuntimeDocker
}

// Create a new ImageTarget with the Dockerfiles rewritten with the injected images.
func injectImageDependencies(iTarget model.ImageTarget, iTargetMap map[model.TargetID]model.ImageTarget, deps []store.BuildResult) (model.ImageTarget, error) {
	if len(deps) == 0 {
		return iTarget, nil
	}

	df := dockerfile.Dockerfile("")
	switch bd := iTarget.BuildDetails.(type) {
	case model.DockerBuild:
		df = dockerfile.Dockerfile(bd.Dockerfile)
	default:
		return model.ImageTarget{}, fmt.Errorf("image %q has no valid buildDetails", iTarget.Refs.ConfigurationRef)
	}

	ast, err := dockerfile.ParseAST(df)
	if err != nil {
		return model.ImageTarget{}, errors.Wrap(err, "injectImageDependencies")
	}

	for _, dep := range deps {
		image := store.LocalImageRefFromBuildResult(dep)
		if image == nil {
			return model.ImageTarget{}, fmt.Errorf("Internal error: image is nil")
		}
		id := dep.TargetID()
		modified, err := ast.InjectImageDigest(iTargetMap[id].Refs.ConfigurationRef, image)
		if err != nil {
			return model.ImageTarget{}, errors.Wrap(err, "injectImageDependencies")
		} else if !modified {
			return model.ImageTarget{}, fmt.Errorf("Could not inject image %q into Dockerfile of image %q", image, iTarget.Refs.ConfigurationRef)
		}
	}

	newDf, err := ast.Print()
	if err != nil {
		return model.ImageTarget{}, errors.Wrap(err, "injectImageDependencies")
	}

	bd := iTarget.DockerBuildInfo()
	bd.Dockerfile = newDf.String()
	iTarget = iTarget.WithBuildDetails(bd)

	return iTarget, nil
}
