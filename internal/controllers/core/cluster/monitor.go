package cluster

import (
	"context"
	"errors"

	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/k8s"
)

func (r *Reconciler) monitorConn(ctx context.Context, clusterNN types.NamespacedName, conn connection) {
	if conn.connType != connectionTypeK8s {
		// live connection monitoring for Docker not yet supported
		return
	}

	ticker := r.clock.NewTicker(clientHealthPollInterval)
	defer ticker.Stop()
	for {
		lastErr := conn.statusError

		err := doKubernetesHealthCheck(ctx, conn.k8sClient)
		if err != nil {
			conn.statusError = err.Error()
		} else {
			conn.statusError = ""
		}

		if conn.statusError != lastErr {
			r.connManager.store(clusterNN, conn)
			r.requeuer.Add(clusterNN)
		}

		select {
		case <-ticker.Chan():
		case <-ctx.Done():
			return
		}
	}
}

func doKubernetesHealthCheck(ctx context.Context, client k8s.Client) error {
	// TODO(milas): use verbose=true and propagate the info to the Tilt API
	// 	cluster obj to show in the web UI
	health, err := client.ClusterHealth(ctx, false)
	if err != nil {
		return err
	}

	if !health.Live {
		return errors.New("cluster did not pass liveness check")
	}

	if !health.Ready {
		return errors.New("cluster not ready")
	}

	return nil
}
