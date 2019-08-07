package engine

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"time"

	"github.com/docker/distribution/reference"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/analytics"
	"github.com/windmilleng/tilt/internal/build"
	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockerfile"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/synclet/sidecar"
)

var _ BuildAndDeployer = &ImageBuildAndDeployer{}

type KINDPusher interface {
	PushToKIND(ctx context.Context, ref reference.NamedTagged, w io.Writer) error
}

type cmdKINDPusher struct{}

func (*cmdKINDPusher) PushToKIND(ctx context.Context, ref reference.NamedTagged, w io.Writer) error {
	cmd := exec.CommandContext(ctx, "kind", "load", "docker-image", ref.String())
	cmd.Stdout = w
	cmd.Stderr = w

	return cmd.Run()
}

func NewKINDPusher() KINDPusher {
	return &cmdKINDPusher{}
}

type ImageBuildAndDeployer struct {
	ib            build.ImageBuilder
	icb           *imageAndCacheBuilder
	k8sClient     k8s.Client
	env           k8s.Env
	runtime       container.Runtime
	analytics     *analytics.TiltAnalytics
	injectSynclet bool
	clock         build.Clock
	kp            KINDPusher
}

func NewImageBuildAndDeployer(
	b build.ImageBuilder,
	cacheBuilder build.CacheBuilder,
	customBuilder build.CustomBuilder,
	k8sClient k8s.Client,
	env k8s.Env,
	analytics *analytics.TiltAnalytics,
	updMode UpdateMode,
	c build.Clock,
	runtime container.Runtime,
	kp KINDPusher,
) *ImageBuildAndDeployer {
	return &ImageBuildAndDeployer{
		ib:        b,
		icb:       NewImageAndCacheBuilder(b, cacheBuilder, customBuilder, updMode),
		k8sClient: k8sClient,
		env:       env,
		analytics: analytics,
		clock:     c,
		runtime:   runtime,
		kp:        kp,
	}
}

// Turn on synclet injection. Should be called before any builds.
func (ibd *ImageBuildAndDeployer) SetInjectSynclet(inject bool) {
	ibd.injectSynclet = inject
}

func (ibd *ImageBuildAndDeployer) BuildAndDeploy(ctx context.Context, st store.RStore, specs []model.TargetSpec, stateSet store.BuildStateSet) (resultSet store.BuildResultSet, err error) {
	iTargets, kTargets := extractImageAndK8sTargets(specs)
	if len(kTargets) == 0 && len(iTargets) == 0 {
		return store.BuildResultSet{}, SilentRedirectToNextBuilderf("ImageBuildAndDeployer does not support these specs")
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

	q, err := NewImageTargetQueue(ctx, iTargets, stateSet, ibd.ib.ImageExists)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	numStages := q.CountDirty() * 2 // each image target has two stages: one for build, and one for push
	if len(kTargets) > 0 {
		numStages++
	}

	ps := build.NewPipelineState(ctx, numStages, ibd.clock)
	defer func() { ps.End(ctx, err) }()

	var anyInPlaceBuild bool

	iTargetMap := model.ImageTargetsByID(iTargets)
	err = q.RunBuilds(func(target model.TargetSpec, state store.BuildState, depResults []store.BuildResult) (store.BuildResult, error) {
		iTarget, ok := target.(model.ImageTarget)
		if !ok {
			return store.BuildResult{}, fmt.Errorf("Not an image target: %T", target)
		}

		iTarget, err := injectImageDependencies(iTarget, iTargetMap, depResults)
		if err != nil {
			return store.BuildResult{}, err
		}

		ref, err := ibd.icb.Build(ctx, iTarget, state, ps)
		if err != nil {
			return store.BuildResult{}, err
		}

		ref, err = ibd.push(ctx, ref, ps, iTarget, kTargets)
		if err != nil {
			return store.BuildResult{}, err
		}

		anyInPlaceBuild = anyInPlaceBuild ||
			!iTarget.AnyFastBuildInfo().Empty() || !iTarget.AnyLiveUpdateInfo().Empty()
		return store.NewImageBuildResult(iTarget.ID(), ref), nil
	})
	if err != nil {
		return store.BuildResultSet{}, err
	}

	// (If we pass an empty list of refs here (as we will do if only deploying
	// yaml), we just don't inject any image refs into the yaml, nbd.
	err = ibd.deploy(ctx, st, ps, iTargetMap, kTargets, q.results, anyInPlaceBuild)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	return q.results, nil
}

func (ibd *ImageBuildAndDeployer) push(ctx context.Context, ref reference.NamedTagged, ps *build.PipelineState, iTarget model.ImageTarget, kTargets []model.K8sTarget) (reference.NamedTagged, error) {
	ps.StartPipelineStep(ctx, "Pushing %s", ref.String())
	defer ps.EndPipelineStep(ctx)

	cbSkip := false
	if iTarget.IsCustomBuild() {
		cbSkip = iTarget.CustomBuildInfo().DisablePush
	}

	// We can also skip the push of the image if it isn't used
	// in any k8s resources! (e.g., it's consumed by another image).
	if ibd.canAlwaysSkipPush() || !isImageDeployedToK8s(iTarget, kTargets) || cbSkip {
		ps.Printf(ctx, "Skipping push")
		return ref, nil
	}

	var err error
	if ibd.env == k8s.EnvKIND {
		ps.Printf(ctx, "Pushing to KIND")
		err := ibd.kp.PushToKIND(ctx, ref, ps.Writer(ctx))
		if err != nil {
			return nil, fmt.Errorf("Error pushing to KIND: %v", err)
		}
	} else {
		ps.Printf(ctx, "Pushing to registry")
		ref, err = ibd.ib.PushImage(ctx, ref, ps.Writer(ctx))
		if err != nil {
			return nil, err
		}
	}

	return ref, nil
}

// Returns: the entities deployed and the namespace of the pod with the given image name/tag.
func (ibd *ImageBuildAndDeployer) deploy(ctx context.Context, st store.RStore, ps *build.PipelineState,
	iTargetMap map[model.TargetID]model.ImageTarget, k8sTargets []model.K8sTarget, results store.BuildResultSet, needsSynclet bool) error {
	ps.StartPipelineStep(ctx, "Deploying")
	defer ps.EndPipelineStep(ctx)

	ps.StartBuildStep(ctx, "Parsing Kubernetes config YAML")

	newK8sEntities := []k8s.K8sEntity{}

	deployID := model.NewDeployID()
	deployLabel := k8s.TiltDeployLabel(deployID)

	var targetIDs []model.TargetID
	var displayNames []string

	for _, k8sTarget := range k8sTargets {
		displayNames = append(displayNames, k8sTarget.DisplayNames...)

		// TODO(nick): The parsed YAML should probably be a part of the model?
		// It doesn't make much sense to re-parse it and inject labels on every deploy.
		entities, err := k8s.ParseYAMLFromString(k8sTarget.YAML)
		if err != nil {
			return err
		}

		depIDs := k8sTarget.DependencyIDs()
		injectedDepIDs := map[model.TargetID]bool{}
		for _, e := range entities {
			injectedSynclet := false
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
				ref := results[depID].Image
				if ref == nil {
					return fmt.Errorf("Internal error: missing build result for dependency ID: %s", depID)
				}

				iTarget := iTargetMap[depID]
				selector := iTarget.ConfigurationRef
				matchInEnvVars := iTarget.MatchInEnvVars

				var replaced bool
				e, replaced, err = k8s.InjectImageDigest(e, selector, ref, matchInEnvVars, policy)
				if err != nil {
					return err
				}
				if replaced {
					injectedDepIDs[depID] = true

					if !iTarget.OverrideCmd.Empty() {
						e, err = k8s.InjectCommand(e, ref, iTarget.OverrideCmd)
						if err != nil {
							return err
						}
					}

					if ibd.injectSynclet && needsSynclet && !injectedSynclet {
						injectedRefSelector := container.NewRefSelector(ref).WithExactMatch()

						var sidecarInjected bool
						e, sidecarInjected, err = sidecar.InjectSyncletSidecar(e, injectedRefSelector)
						if err != nil {
							return err
						}
						if !sidecarInjected {
							return fmt.Errorf("Could not inject synclet: %v", e)
						}
						injectedSynclet = true
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

	l := logger.Get(ctx)
	writer := logger.NewPrefixedWriter(logger.Blue(l).Sprint("  │ "), l.Writer(logger.InfoLvl))
	l = logger.NewLogger(logger.InfoLvl, writer)

	l.Infof("Applying via kubectl:")
	for _, displayName := range displayNames {
		l.Infof("   %s", displayName)
	}

	return ibd.k8sClient.Upsert(logger.WithLogger(ctx, l), newK8sEntities)
}

// If we're using docker-for-desktop as our k8s backend,
// we don't need to push to the central registry.
// The k8s will use the image already available
// in the local docker daemon.
func (ibd *ImageBuildAndDeployer) canAlwaysSkipPush() bool {
	return ibd.env.IsLocalCluster() && ibd.runtime == container.RuntimeDocker
}

// Create a new ImageTarget with the dockerfiles rewritten
// with the injected images.
func injectImageDependencies(iTarget model.ImageTarget, iTargetMap map[model.TargetID]model.ImageTarget, deps []store.BuildResult) (model.ImageTarget, error) {
	if len(deps) == 0 {
		return iTarget, nil
	}

	df := dockerfile.Dockerfile("")
	switch bd := iTarget.BuildDetails.(type) {
	case model.DockerBuild:
		df = dockerfile.Dockerfile(bd.Dockerfile)
	case model.FastBuild:
		df = dockerfile.Dockerfile(bd.BaseDockerfile)
	default:
		return model.ImageTarget{}, fmt.Errorf("image %q has no valid buildDetails", iTarget.ConfigurationRef)
	}

	ast, err := dockerfile.ParseAST(df)
	if err != nil {
		return model.ImageTarget{}, errors.Wrap(err, "injectImageDependencies")
	}

	for _, dep := range deps {
		modified, err := ast.InjectImageDigest(iTargetMap[dep.TargetID].ConfigurationRef, dep.Image)
		if err != nil {
			return model.ImageTarget{}, errors.Wrap(err, "injectImageDependencies")
		} else if !modified {
			return model.ImageTarget{}, fmt.Errorf("Could not inject image %q into Dockerfile of image %q", dep.Image, iTarget.ConfigurationRef)
		}
	}

	newDf, err := ast.Print()
	if err != nil {
		return model.ImageTarget{}, errors.Wrap(err, "injectImageDependencies")
	}

	switch bd := iTarget.BuildDetails.(type) {
	case model.DockerBuild:
		bd.Dockerfile = newDf.String()
		iTarget = iTarget.WithBuildDetails(bd)
	case model.FastBuild:
		bd.BaseDockerfile = newDf.String()
		iTarget = iTarget.WithBuildDetails(bd)
	}

	return iTarget, nil
}
