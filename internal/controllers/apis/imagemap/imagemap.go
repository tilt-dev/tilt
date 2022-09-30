package imagemap

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/types"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Inject completed ImageMaps into the environment of a local process that
// wants to deploy to a cluster.
//
// Current env vars:
// TILT_IMAGE_i - The reference to the image #i from the point of view of the cluster container runtime.
// TILT_IMAGE_MAP_i - The name of the image map #i with the current status of the image.
//
// where an env may depend on arbitrarily many image maps.
func InjectIntoDeployEnv(cmd *model.Cmd, imageMapNames []string, imageMaps map[types.NamespacedName]*v1alpha1.ImageMap) error {
	for i, imageMapName := range imageMapNames {
		imageMap, ok := imageMaps[types.NamespacedName{Name: imageMapName}]
		if !ok {
			return fmt.Errorf("internal error: missing imagemap %s", imageMapName)
		}

		cmd.Env = append(cmd.Env, fmt.Sprintf("TILT_IMAGE_MAP_%d=%s", i, imageMapName))
		cmd.Env = append(cmd.Env, fmt.Sprintf("TILT_IMAGE_%d=%s", i, imageMap.Status.ImageFromCluster))
	}
	return nil
}

// Inject completed ImageMaps into the environment of a local process that
// wants to build images locally.
//
// Current env vars:
// TILT_IMAGE_i - The reference to the image #i from the point of view of the local host.
// TILT_IMAGE_MAP_i - The name of the image map #i with the current status of the image.
//
// where an env may depend on arbitrarily many image maps.
func InjectIntoLocalEnv(cmd *v1alpha1.Cmd, imageMapNames []string, imageMaps map[types.NamespacedName]*v1alpha1.ImageMap) (*v1alpha1.Cmd, error) {
	cmd = cmd.DeepCopy()
	for i, imageMapName := range imageMapNames {
		imageMap, ok := imageMaps[types.NamespacedName{Name: imageMapName}]
		if !ok {
			return nil, fmt.Errorf("internal error: missing imagemap %s", imageMapName)
		}

		cmd.Spec.Env = append(cmd.Spec.Env, fmt.Sprintf("TILT_IMAGE_MAP_%d=%s", i, imageMapName))
		cmd.Spec.Env = append(cmd.Spec.Env, fmt.Sprintf("TILT_IMAGE_%d=%s", i, imageMap.Status.ImageFromLocal))
	}
	return cmd, nil
}

// Populate a map with all the given imagemaps, skipping any that don't exist
func NamesToObjects(ctx context.Context, client ctrlclient.Client, names []string) (map[types.NamespacedName]*v1alpha1.ImageMap, error) {
	imageMaps := make(map[types.NamespacedName]*v1alpha1.ImageMap)
	for _, name := range names {
		var im v1alpha1.ImageMap
		nn := types.NamespacedName{Name: name}
		err := client.Get(ctx, nn, &im)
		if err != nil {
			if apierrors.IsNotFound(err) {
				// If the map isn't found, keep going
				continue
			}
			return nil, err
		}

		imageMaps[nn] = &im
	}
	return imageMaps, nil
}
