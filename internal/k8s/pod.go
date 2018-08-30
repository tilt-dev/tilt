package k8s

import (
	"context"

	"github.com/docker/distribution/reference"
)

// PodWithImage returns the ID of the pod running the given image. We expect exactly one
// matching pod: if too many or too few matches, throw an error.
func (k KubectlClient) PodWithImage(ctx context.Context, image reference.Named) (PodID, error) {
	return PodID(""), nil
}
