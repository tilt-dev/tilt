package cluster

import (
	"github.com/google/wire"
	"github.com/spf13/afero"

	"github.com/tilt-dev/tilt/internal/controllers/apis/cluster"
)

var WireSet = wire.NewSet(
	NewConnectionManager,
	wire.Bind(new(cluster.ClientProvider), new(*ConnectionManager)),
	wire.InterfaceValue(new(KubernetesClientFactory), KubernetesClientFunc(KubernetesClientFromEnv)),
	wire.InterfaceValue(new(DockerClientFactory), DockerClientFunc(DockerClientFromEnv)),
	afero.NewOsFs,
)
