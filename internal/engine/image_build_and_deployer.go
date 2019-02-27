package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockerfile"
	"github.com/windmilleng/tilt/internal/store"

	"github.com/pkg/errors"

	"github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/synclet/sidecar"
	"github.com/windmilleng/wmclient/pkg/analytics"
	"k8s.io/api/core/v1"
)

var _ BuildAndDeployer = &ImageBuildAndDeployer{}

type ImageBuildAndDeployer struct {
	icb           *imageAndCacheBuilder
	k8sClient     k8s.Client
	env           k8s.Env
	runtime       container.Runtime
	analytics     analytics.Analytics
	injectSynclet bool
	clock         build.Clock
}

func NewImageBuildAndDeployer(
	b build.ImageBuilder,
	cacheBuilder build.CacheBuilder,
	customBuilder build.CustomBuilder,
	k8sClient k8s.Client,
	env k8s.Env,
	analytics analytics.Analytics,
	updMode UpdateMode,
	c build.Clock,
	runtime container.Runtime) *ImageBuildAndDeployer {
	return &ImageBuildAndDeployer{
		icb:       NewImageAndCacheBuilder(b, cacheBuilder, customBuilder, updMode),
		k8sClient: k8sClient,
		env:       env,
		analytics: analytics,
		clock:     c,
		runtime:   runtime,
	}
}

// Turn on synclet injection. Should be called before any builds.
func (ibd *ImageBuildAndDeployer) SetInjectSynclet(inject bool) {
	ibd.injectSynclet = inject
}

func (ibd *ImageBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, stateSet store.BuildStateSet) (resultSet store.BuildResultSet, err error) {
	iTargets, kTargets := extractImageAndK8sTargets(specs)
	if len(kTargets) == 0 && len(iTargets) == 0 {
		return store.BuildResultSet{}, RedirectToNextBuilderf("ImageBuildAndDeployer does not support these specs")
	}

	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-ImageBuildAndDeployer-BuildAndDeploy")
	span.SetTag("target", kTargets[0].Name)
	defer span.Finish()

	startTime := time.Now()
	defer func() {
		incremental := "0"
		for _, state := range stateSet {
			if state.HasImage() {
				incremental = "1"
			}
		}
		tags := map[string]string{"incremental": incremental}
		ibd.analytics.Timer("build.image", time.Since(startTime), tags)
	}()

	numStages := len(iTargets)
	if len(kTargets) > 0 {
		numStages++
	}

	ps := build.NewPipelineState(ctx, numStages, ibd.clock)
	defer func() { ps.End(ctx, err) }()

	var anyFastBuild bool
	q := NewImageTargetQueue(iTargets)
	target, ok, err := q.Next()
	if err != nil {
		return store.BuildResultSet{}, err
	}

	for ok {
		iTarget, err := injectImageDependencies(target.(model.ImageTarget), q.DependencyResults(target))
		if err != nil {
			return store.BuildResultSet{}, err
		}

		// TODO(nick): We can also skip the push of the image if it isn't used
		// in any k8s resources! (e.g., it's consumed by another image).
		ref, err := ibd.icb.Build(ctx, iTarget, stateSet[iTarget.ID()], ps, ibd.canSkipPush())
		if err != nil {
			return store.BuildResultSet{}, err
		}

		q.SetResult(iTarget.ID(), store.BuildResult{Image: ref})
		anyFastBuild = anyFastBuild || iTarget.IsFastBuild()
		target, ok, err = q.Next()
		if err != nil {
			return store.BuildResultSet{}, err
		}
	}

	// (If we pass an empty list of refs here (as we will do if only deploying
	// yaml), we just don't inject any image refs into the yaml, nbd.
	err = ibd.deploy(ctx, st, ps, kTargets, q.results, anyFastBuild)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	return q.results, nil
}

// Returns: the entities deployed and the namespace of the pod with the given image name/tag.
func (ibd *ImageBuildAndDeployer) deploy(ctx context.Context, st store.RStore, ps *build.PipelineState, k8sTargets []model.K8sTarget, results store.BuildResultSet, needsSynclet bool) error {
	ps.StartPipelineStep(ctx, "Deploying")
	defer ps.EndPipelineStep(ctx)

	ps.StartBuildStep(ctx, "Parsing Kubernetes config YAML")

	newK8sEntities := []k8s.K8sEntity{}

	deployID := model.NewDeployID()
	deployLabel := k8s.TiltDeployLabel(deployID)

	var targetIDs []model.TargetID

	for _, k8sTarget := range k8sTargets {
		// TODO(nick): The parsed YAML should probably be a part of the model?
		// It doesn't make much sense to re-parse it and inject labels on every deploy.
		entities, err := k8s.ParseYAMLFromString(k8sTarget.YAML)
		if err != nil {
			return err
		}

		depIDs := k8sTarget.DependencyIDs()
		injectedDepIDs := map[model.TargetID]bool{}
		for _, e := range entities {
			e, err = k8s.InjectLabels(e, []model.LabelPair{k8s.TiltRunLabel(), {Key: k8s.ManifestNameLabel, Value: k8sTarget.Name.String()}, deployLabel})
			if err != nil {
				return errors.Wrap(err, "deploy")
			}

			// For development, image pull policy should never be set to "Always",
			// even if it might make sense to use "Always" in prod. People who
			// set "Always" for development are shooting their own feet.
			e, err = k8s.InjectImagePullPolicy(e, v1.PullIfNotPresent)
			if err != nil {
				return err
			}

			// When working with a local k8s cluster, we set the pull policy to Never,
			// to ensure that k8s fails hard if the image is missing from docker.
			policy := v1.PullIfNotPresent
			if ibd.canSkipPush() {
				policy = v1.PullNever
			}

			for _, depID := range depIDs {
				ref := results[depID].Image
				if ref == nil {
					return fmt.Errorf("Internal error: missing build result for dependency ID: %s", depID)
				}

				var replaced bool
				e, replaced, err = k8s.InjectImageDigest(e, ref, policy)
				if err != nil {
					return err
				}
				if replaced {
					injectedDepIDs[depID] = true

					if ibd.injectSynclet && needsSynclet {
						var sidecarInjected bool
						e, sidecarInjected, err = sidecar.InjectSyncletSidecar(e, ref)
						if err != nil {
							return err
						}
						if !sidecarInjected {
							return fmt.Errorf("Could not inject synclet: %v", e)
						}
					}
				}
			}
			newK8sEntities = append(newK8sEntities, e)
		}
		targetIDs = append(targetIDs, k8sTarget.ID())

		for _, depID := range depIDs {
			if !injectedDepIDs[depID] {
				return fmt.Errorf("Docker image missing from yaml: %s", depID)
			}
		}
	}

	deployIDActions := NewDeployIDActionsForTargets(targetIDs, deployID)
	for _, a := range deployIDActions {
		st.Dispatch(a)
	}

	return ibd.k8sClient.Upsert(ctx, newK8sEntities)
}

// If we're using docker-for-desktop as our k8s backend,
// we don't need to push to the central registry.
// The k8s will use the image already available
// in the local docker daemon.
func (ibd *ImageBuildAndDeployer) canSkipPush() bool {
	return ibd.env.IsLocalCluster() && ibd.runtime == container.RuntimeDocker
}

// Create a new ImageTarget with the dockerfiles rewritten
// with the injected images.
func injectImageDependencies(iTarget model.ImageTarget, deps []store.BuildResult) (model.ImageTarget, error) {
	if len(deps) == 0 {
		return iTarget, nil
	}

	df := dockerfile.Dockerfile("")
	switch bd := iTarget.BuildDetails.(type) {
	case model.StaticBuild:
		df = dockerfile.Dockerfile(bd.Dockerfile)
	case model.FastBuild:
		df = dockerfile.Dockerfile(bd.BaseDockerfile)
	default:
		return model.ImageTarget{}, fmt.Errorf("image %q has no valid buildDetails", iTarget.Ref)
	}

	ast, err := dockerfile.ParseAST(df)
	if err != nil {
		return model.ImageTarget{}, errors.Wrap(err, "injectImageDependencies")
	}

	for _, dep := range deps {
		modified, err := ast.InjectImageDigest(dep.Image)
		if err != nil {
			return model.ImageTarget{}, errors.Wrap(err, "injectImageDependencies")
		} else if !modified {
			return model.ImageTarget{}, fmt.Errorf("Could not inject image %q into Dockerfile of image %q", dep.Image, iTarget.Ref)
		}
	}

	newDf, err := ast.Print()
	if err != nil {
		return model.ImageTarget{}, errors.Wrap(err, "injectImageDependencies")
	}

	switch bd := iTarget.BuildDetails.(type) {
	case model.StaticBuild:
		bd.Dockerfile = newDf.String()
		iTarget = iTarget.WithBuildDetails(bd)
	case model.FastBuild:
		bd.BaseDockerfile = newDf.String()
		iTarget = iTarget.WithBuildDetails(bd)
	}

	return iTarget, nil
}
