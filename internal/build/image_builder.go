package build

import (
	"context"
	"errors"
	"fmt"

	"github.com/docker/distribution/reference"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/clusterid"
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/ignore"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

type ImageBuilder struct {
	db    *DockerBuilder
	custb *CustomBuilder
	kl    KINDLoader
}

func NewImageBuilder(db *DockerBuilder, custb *CustomBuilder, kl KINDLoader) *ImageBuilder {
	return &ImageBuilder{
		db:    db,
		custb: custb,
		kl:    kl,
	}
}

func (ib *ImageBuilder) CanReuseRef(ctx context.Context, iTarget model.ImageTarget, ref reference.NamedTagged) (bool, error) {
	switch iTarget.BuildDetails.(type) {
	case model.DockerBuild:
		return ib.db.ImageExists(ctx, ref)
	case model.CustomBuild:
		// Custom build doesn't have a good way to check if the ref still exists in the image
		// store, so just assume we can.
		return true, nil
	}
	return false, fmt.Errorf("image %q has no valid buildDetails (neither "+
		"DockerBuild nor CustomBuild)", iTarget.ImageMapSpec.Selector)
}

// Build the image, and push it if necessary.
//
// Note that this function can return partial results on an error.
//
// The error is simply the "main" build failure reason.
func (ib *ImageBuilder) Build(ctx context.Context,
	iTarget model.ImageTarget,
	cluster *v1alpha1.Cluster,
	imageMaps map[types.NamespacedName]*v1alpha1.ImageMap,
	ps *PipelineState) (container.TaggedRefs, []v1alpha1.DockerImageStageStatus, error) {
	refs, stages, err := ib.buildOnly(ctx, iTarget, cluster, imageMaps, ps)
	if err != nil {
		return refs, stages, err
	}

	pushStage := ib.push(ctx, refs, ps, iTarget, cluster)
	if pushStage != nil {
		stages = append(stages, *pushStage)
	}

	if pushStage != nil && pushStage.Error != "" {
		err = errors.New(pushStage.Error)
	}

	return refs, stages, err
}

// Build the image, but don't do any push.
func (ib *ImageBuilder) buildOnly(ctx context.Context,
	iTarget model.ImageTarget,
	cluster *v1alpha1.Cluster,
	imageMaps map[types.NamespacedName]*v1alpha1.ImageMap,
	ps *PipelineState,
) (container.TaggedRefs, []v1alpha1.DockerImageStageStatus, error) {
	refs, err := iTarget.Refs(cluster)
	if err != nil {
		return container.TaggedRefs{}, nil, err
	}

	userFacingRefName := container.FamiliarString(refs.ConfigurationRef)

	switch bd := iTarget.BuildDetails.(type) {
	case model.DockerBuild:
		ps.StartPipelineStep(ctx, "Building Dockerfile: [%s]", userFacingRefName)
		defer ps.EndPipelineStep(ctx)

		return ib.db.BuildImage(ctx, ps, refs, bd.DockerImageSpec,
			cluster,
			imageMaps,
			ignore.CreateBuildContextFilter(iTarget))

	case model.CustomBuild:
		ps.StartPipelineStep(ctx, "Building Custom Build: [%s]", userFacingRefName)
		defer ps.EndPipelineStep(ctx)
		refs, err := ib.custb.Build(ctx, refs, bd.CmdImageSpec, imageMaps)
		return refs, nil, err
	}

	// Theoretically this should never trip b/c we `validate` the manifest beforehand...?
	// If we get here, something is very wrong.
	return container.TaggedRefs{}, nil, fmt.Errorf("image %q has no valid buildDetails (neither "+
		"DockerBuild nor CustomBuild)", refs.ConfigurationRef)
}

// Push the image if the cluster requires it.
func (ib *ImageBuilder) push(ctx context.Context, refs container.TaggedRefs, ps *PipelineState, iTarget model.ImageTarget, cluster *v1alpha1.Cluster) *v1alpha1.DockerImageStageStatus {
	// Skip the push phase entirely if we're on Docker Compose.
	isDC := cluster != nil &&
		cluster.Spec.Connection != nil &&
		cluster.Spec.Connection.Docker != nil
	if isDC {
		return nil
	}

	// On Kubernetes, we count each push() as a stage, and need to print why
	// we're skipping if we don't need to push.
	ps.StartPipelineStep(ctx, "Pushing %s", container.FamiliarString(refs.LocalRef))
	defer ps.EndPipelineStep(ctx)

	cbSkip := false
	if iTarget.IsCustomBuild() {
		cbSkip = iTarget.CustomBuildInfo().SkipsPush()
	}

	if cbSkip {
		ps.Printf(ctx, "Skipping push: custom_build() configured to handle push itself")
		return nil
	}

	// We can also skip the push of the image if it isn't used
	// in any k8s resources! (e.g., it's consumed by another image).
	if iTarget.ClusterNeeds() != v1alpha1.ClusterImageNeedsPush {
		ps.Printf(ctx, "Skipping push: base image does not need deploy")
		return nil
	}

	if ib.db.WillBuildToKubeContext(k8s.KubeContext(k8sConnStatus(cluster).Context)) {
		ps.Printf(ctx, "Skipping push: building on cluster's container runtime")
		return nil
	}

	startTime := apis.NowMicro()
	var err error
	if ib.shouldUseKINDLoad(refs, cluster) {
		ps.Printf(ctx, "Loading image to KIND")
		err := ib.kl.LoadToKIND(ps.AttachLogger(ctx), cluster, refs.LocalRef)
		endTime := apis.NowMicro()
		stage := &v1alpha1.DockerImageStageStatus{
			Name:       "kind load",
			StartedAt:  &startTime,
			FinishedAt: &endTime,
		}
		if err != nil {
			stage.Error = fmt.Sprintf("Error loading image to KIND: %v", err)
		}
		return stage
	}

	ps.Printf(ctx, "Pushing with Docker client")
	err = ib.db.PushImage(ps.AttachLogger(ctx), refs.LocalRef)

	endTime := apis.NowMicro()
	stage := &v1alpha1.DockerImageStageStatus{
		Name:       "docker push",
		StartedAt:  &startTime,
		FinishedAt: &endTime,
	}
	if err != nil {
		stage.Error = fmt.Sprintf("docker push: %v", err)
	}
	return stage
}

func (ib *ImageBuilder) shouldUseKINDLoad(refs container.TaggedRefs, cluster *v1alpha1.Cluster) bool {
	isKIND := k8sConnStatus(cluster).Product == string(clusterid.ProductKIND)
	if !isKIND {
		return false
	}

	// if we're using KIND and the image has a separate ref by which it's referred to
	// in the cluster, that implies that we have a local registry in place, and should
	// push to that instead of using KIND load.
	if refs.LocalRef.String() != refs.ClusterRef.String() {
		return false
	}

	hasRegistry := cluster.Status.Registry != nil && cluster.Status.Registry.Host != ""
	if hasRegistry {
		return false
	}

	return true
}

func k8sConnStatus(cluster *v1alpha1.Cluster) *v1alpha1.KubernetesClusterConnectionStatus {
	if cluster != nil &&
		cluster.Status.Connection != nil &&
		cluster.Status.Connection.Kubernetes != nil {
		return cluster.Status.Connection.Kubernetes
	}
	return &v1alpha1.KubernetesClusterConnectionStatus{}
}
