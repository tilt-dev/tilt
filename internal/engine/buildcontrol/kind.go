package buildcontrol

import (
	"context"
	"os/exec"
	"strings"

	"github.com/docker/distribution/reference"

	"github.com/tilt-dev/clusterid"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
)

type KINDLoader interface {
	LoadToKIND(ctx context.Context, cluster *v1alpha1.Cluster, ref reference.NamedTagged) error
}

type cmdKINDLoader struct {
}

func (kl *cmdKINDLoader) LoadToKIND(ctx context.Context, cluster *v1alpha1.Cluster, ref reference.NamedTagged) error {
	// In Kind5, --name specifies the name of the cluster in the kubeconfig.
	// In Kind6, the -name parameter is prefixed with 'kind-' before being written to/read from the kubeconfig
	k8sConn := k8sConnStatus(cluster)
	kindName := k8sConn.Cluster
	if k8sConn.Product == string(clusterid.ProductKIND) {
		kindName = strings.TrimPrefix(kindName, "kind-")
	}

	cmd := exec.CommandContext(ctx, "kind", "load", "docker-image", ref.String(), "--name", kindName)
	w := logger.NewMutexWriter(logger.Get(ctx).Writer(logger.InfoLvl))
	cmd.Stdout = w
	cmd.Stderr = w

	return cmd.Run()
}

func NewKINDLoader() KINDLoader {
	return &cmdKINDLoader{}
}
