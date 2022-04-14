package cluster

import (
	"github.com/tilt-dev/tilt/internal/container"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func RegistryHosting(registry *container.Registry) *v1alpha1.RegistryHosting {
	if registry == nil || registry.Empty() {
		return nil
	}

	return &v1alpha1.RegistryHosting{
		Host:                     registry.Host,
		HostFromContainerRuntime: registry.HostFromCluster(),
		SingleName:               registry.SingleName,
	}
}
