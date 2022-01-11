package tiltfile

import (
	"context"

	dockertypes "github.com/docker/docker/api/types"

	"github.com/tilt-dev/tilt/internal/analytics"
)

// reportDockerConnectionEvent records a metric about Docker connectivity.
func (r *Reconciler) reportDockerConnectionEvent(ctx context.Context, success bool, serverVersion dockertypes.Version) {
	r.dockerConnectMetricReporter.Do(func() {
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
