package cluster

import (
	"context"
	"errors"
	"sync"

	"github.com/jonboulle/clockwork"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/controllers/indexer"
	"github.com/tilt-dev/tilt/internal/k8s"
)

type clusterHealthMonitor struct {
	mu        sync.Mutex
	globalCtx context.Context
	clock     clockwork.Clock
	requeuer  *indexer.Requeuer
	monitors  map[types.NamespacedName]monitor
}

func newClusterHealthMonitor(globalCtx context.Context, clock clockwork.Clock, requeuer *indexer.Requeuer) *clusterHealthMonitor {
	return &clusterHealthMonitor{
		globalCtx: globalCtx,
		clock:     clock,
		requeuer:  requeuer,
		monitors:  make(map[types.NamespacedName]monitor),
	}
}

func (c *clusterHealthMonitor) Start(clusterNN types.NamespacedName, conn connection) context.Context {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cleanup(clusterNN)
	ctx, cancel := context.WithCancel(c.globalCtx)
	c.monitors[clusterNN] = monitor{cancel: cancel}
	go c.run(ctx, clusterNN, conn)

	return ctx
}

func (c *clusterHealthMonitor) GetStatus(clusterNN types.NamespacedName) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.monitors[clusterNN].error
}

func (c *clusterHealthMonitor) UpdateStatus(ctx context.Context, clusterNN types.NamespacedName, error string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if ctx.Err() != nil {
		// if the context as canceled while the health check was running,
		// it might be the cause of the error, which isn't actually a health
		// check failure; it's also possible we'd be doing a stale update
		return
	}

	if m, ok := c.monitors[clusterNN]; ok {
		if m.error == error {
			return
		}
		m.error = error
		c.monitors[clusterNN] = m
		c.requeuer.Add(clusterNN)
	}
}

func (c *clusterHealthMonitor) Stop(clusterNN types.NamespacedName) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.cleanup(clusterNN)
	delete(c.monitors, clusterNN)
}

func (c *clusterHealthMonitor) cleanup(clusterNN types.NamespacedName) {
	m := c.monitors[clusterNN]
	if m.cancel != nil {
		m.cancel()
	}
}

type monitor struct {
	cancel context.CancelFunc
	error  string
}

func (c *clusterHealthMonitor) run(ctx context.Context, clusterNN types.NamespacedName, conn connection) {
	if conn.connType != connectionTypeK8s {
		// live connection monitoring for Docker not yet supported
		return
	}

	ticker := c.clock.NewTicker(clientHealthPollInterval)
	defer ticker.Stop()
	for {
		err := doKubernetesHealthCheck(ctx, conn.k8sClient)
		if err != nil {
			c.UpdateStatus(ctx, clusterNN, err.Error())
		} else {
			c.UpdateStatus(ctx, clusterNN, "")
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
