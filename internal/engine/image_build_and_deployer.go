package engine

import (
	"context"
	"fmt"
	"time"

	"github.com/windmilleng/tilt/internal/store"

	"github.com/pkg/errors"

	"github.com/docker/distribution/reference"
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
	analytics     analytics.Analytics
	injectSynclet bool
	clock         build.Clock
}

func NewImageBuildAndDeployer(
	b build.ImageBuilder,
	cacheBuilder build.CacheBuilder,
	k8sClient k8s.Client,
	env k8s.Env,
	analytics analytics.Analytics,
	updMode UpdateMode,
	c build.Clock) *ImageBuildAndDeployer {
	return &ImageBuildAndDeployer{
		icb:       NewImageAndCacheBuilder(b, cacheBuilder, updMode),
		k8sClient: k8sClient,
		env:       env,
		analytics: analytics,
		clock:     c,
	}
}

// Turn on synclet injection. Should be called before any builds.
func (ibd *ImageBuildAndDeployer) SetInjectSynclet(inject bool) {
	ibd.injectSynclet = inject
}

func (ibd *ImageBuildAndDeployer) BuildAndDeploy(ctx context.Context, specs []model.TargetSpec, stateSet store.BuildStateSet) (resultSet store.BuildResultSet, err error) {
	iTargets, kTargets := extractImageAndK8sTargets(specs)
	if len(kTargets) == 0 || len(iTargets) == 0 {
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

	results := store.BuildResultSet{}

	var refs []reference.NamedTagged
	for _, iTarget := range iTargets {
		ref, err := ibd.icb.Build(ctx, iTarget, stateSet[iTarget.ID()], ps, ibd.canSkipPush())
		if err != nil {
			return store.BuildResultSet{}, err
		}
		results[iTarget.ID()] = store.BuildResult{
			Image: ref,
		}
		refs = append(refs, ref)
	}

	err = ibd.deploy(ctx, ps, kTargets, refs)
	if err != nil {
		return store.BuildResultSet{}, err
	}

	return results, nil
}

// Returns: the entities deployed and the namespace of the pod with the given image name/tag.
func (ibd *ImageBuildAndDeployer) deploy(ctx context.Context, ps *build.PipelineState, k8sTargets []model.K8sTarget, refs []reference.NamedTagged) error {
	ps.StartPipelineStep(ctx, "Deploying")
	defer ps.EndPipelineStep(ctx)

	ps.StartBuildStep(ctx, "Parsing Kubernetes config YAML")

	injectedRefs := map[string]bool{}
	newK8sEntities := []k8s.K8sEntity{}

	for _, k8sTarget := range k8sTargets {
		// TODO(nick): The parsed YAML should probably be a part of the model?
		// It doesn't make much sense to re-parse it and inject labels on every deploy.
		entities, err := k8s.ParseYAMLFromString(k8sTarget.YAML)
		if err != nil {
			return err
		}

		for _, e := range entities {
			e, err = k8s.InjectLabels(e, []model.LabelPair{TiltRunLabel(), {Key: ManifestNameLabel, Value: k8sTarget.Name.String()}})
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

			for _, ref := range refs {
				var replaced bool
				e, replaced, err = k8s.InjectImageDigest(e, ref, policy)
				if err != nil {
					return err
				}
				if replaced {
					injectedRefs[ref.String()] = true

					if ibd.injectSynclet {
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
	}

	for _, ref := range refs {
		if !injectedRefs[ref.String()] {
			return fmt.Errorf("Docker image missing from yaml: %s", ref)
		}
	}

	return ibd.k8sClient.Upsert(ctx, newK8sEntities)
}

// If we're using docker-for-desktop as our k8s backend,
// we don't need to push to the central registry.
// The k8s will use the image already available
// in the local docker daemon.
func (ibd *ImageBuildAndDeployer) canSkipPush() bool {
	return ibd.env.IsLocalCluster()
}
