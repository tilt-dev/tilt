package dockerimage

import (
	"context"
	"fmt"

	"github.com/docker/distribution/reference"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// A helper function for updating the imagemap
// from dockerimage.Reconciler and cmdimage.Reconciler.
// This is mainly for easing the transition to reconcilers.
func UpdateImageMap(
	ctx context.Context,
	client ctrlclient.Client,
	docker docker.Client,
	iTarget model.ImageTarget,
	cluster *v1alpha1.Cluster,
	imageMaps map[types.NamespacedName]*v1alpha1.ImageMap,
	startTime *metav1.MicroTime,
	refs container.TaggedRefs) (store.ImageBuildResult, error) {

	result := store.NewImageBuildResult(iTarget.ID(), refs.LocalRef, refs.ClusterRef)
	if isDockerCompose(cluster) {
		expectedRef := iTarget.Refs.ConfigurationRef
		ref, err := tagWithExpected(ctx, docker, refs.LocalRef, expectedRef)
		if err != nil {
			return store.ImageBuildResult{}, err
		}

		result = store.NewImageBuildResultSingleRef(iTarget.ID(), ref)
	}

	result.ImageMapStatus.BuildStartTime = startTime
	nn := types.NamespacedName{Name: iTarget.ImageMapName()}
	im, ok := imageMaps[nn]
	if !ok {
		return store.ImageBuildResult{}, fmt.Errorf("apiserver missing ImageMap: %s", iTarget.ID().Name)
	}
	im.Status = result.ImageMapStatus
	err := client.Status().Update(ctx, im)
	if err != nil {
		return store.ImageBuildResult{}, fmt.Errorf("updating ImageMap: %v", err)
	}

	return result, err
}

// tagWithExpected tags the given ref as whatever Docker Compose expects, i.e. as
// the `image` value given in docker-compose.yaml. (If DC yaml specifies an image
// with a tag, use that name + tag; otherwise, tag as latest.)
func tagWithExpected(
	ctx context.Context,
	dCli docker.Client,
	ref reference.NamedTagged,
	expected container.RefSelector) (reference.NamedTagged, error) {
	dCli = dCli.ForOrchestrator(model.OrchestratorDC)

	var tagAs reference.NamedTagged
	expectedNt, err := container.ParseNamedTagged(expected.String())
	if err == nil {
		// expected ref already includes a tag, so just tag the image as that
		tagAs = expectedNt
	} else {
		// expected ref is just a name, so tag it as `latest` b/c that's what Docker Compose wants
		tagAs, err = reference.WithTag(ref, docker.TagLatest)
		if err != nil {
			return nil, err
		}
	}

	err = dCli.ImageTag(ctx, ref.String(), tagAs.String())
	return tagAs, err
}

func isDockerCompose(cluster *v1alpha1.Cluster) bool {
	return cluster != nil &&
		cluster.Spec.Connection != nil &&
		cluster.Spec.Connection.Docker != nil
}
