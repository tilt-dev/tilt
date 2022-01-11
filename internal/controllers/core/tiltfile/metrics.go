package tiltfile

import (
	"context"
	"sync"

	dockertypes "github.com/docker/docker/api/types"

	"github.com/tilt-dev/tilt/internal/analytics"
)

// dockerConnectMetricOnce ensures we only report a single Docker connect status
// event per `tilt up`. Currently, a client is initialized on start (via wire/DI)
// and if there's an error, an exploding client is created; we'll never attempt
// to make a new one after that, so reporting on subsequent Tiltfile loads is
// not useful, as there's no way its status can change currently (a restart of
// Tilt is required).
var dockerConnectMetricOnce sync.Once

// reportDockerConnectionEvent records a metric about Docker connectivity.
func reportDockerConnectionEvent(ctx context.Context, success bool, serverVersion dockertypes.Version) {
	dockerConnectMetricOnce.Do(func() {
		var status string
		if success {
			status = "connected"
		} else {
			status = "error"
		}

		tags := map[string]string{
			"status": status,
		}

		if serverVersion.Version != "" {
			tags["server.version"] = serverVersion.Version
		}

		if serverVersion.Arch != "" {
			tags["server.arch"] = serverVersion.Arch
		}

		analytics.Get(ctx).Incr("api.tiltfile.docker.connect", tags)
	})
}
