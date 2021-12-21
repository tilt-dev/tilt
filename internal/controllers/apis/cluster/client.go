package cluster

import (
	"errors"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/timecmp"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// NotFoundError indicates there is no cluster client for the given key.
var NotFoundError = errors.New("cluster client does not exist")

// StaleClientError indicates that the Cluster object is out-of-date.
//
// This is indicative of a bug in the caller!
//
// If calls to ClientManager::Refresh are not made properly, StaleClientError
// can be returned by ClientManager::GetK8sClient. This is to prevent
// unintentional misuse resulting in a stale client being used while
// also ensuring that callers handle connection changes uniformly.
//
// See the expected usage/flow on ClientManager.
var StaleClientError = errors.New("cluster hash is stale")

// ClientProvider provides cached clients to the ClientManager.
//
// All clients are goroutine-safe.
type ClientProvider interface {
	// GetK8sClient returns the Kubernetes client for the cluster or an error for unknown clusters, connections
	// in a transient error state, or if the connection is of a different type (i.e. Docker Compose).
	//
	// In addition to the client, the timestamp at which the client was created is returned so callers can track
	// when the client instance has changed.
	GetK8sClient(clusterKey types.NamespacedName) (k8s.Client, time.Time, error)
}

type watermarkedClient struct {
	connectedAt time.Time
	client      k8s.Client
}

// ClientManager is a convenience wrapper over ClientProvider which simplifies
// handling client changes.
//
// On reconcile, the controller should:
// 	(1) Fetch the Cluster object referenced by the type its reconciling.
// 	(2) Call ClientManager::Refresh to determine if the client for the cluster
// 		has changed. If true, all state associated with the old cluster should
// 		be cleared.
// 	(3) As needed, call ClientManager::GetK8sClient to get a client instance.
type ClientManager struct {
	mu       sync.Mutex
	provider ClientProvider

	clients map[types.NamespacedName]watermarkedClient
}

func NewClientManager(cache ClientProvider) *ClientManager {
	return &ClientManager{
		provider: cache,
		clients:  make(map[types.NamespacedName]watermarkedClient),
	}
}

// GetK8sClient returns the client associated with the Cluster object.
//
// If no client is known for the Cluster object, NotFoundError is returned.
// If the Cluster object's config hash in the status does not match the known client, StaleClientError is returned.
func (c *ClientManager) GetK8sClient(cluster *v1alpha1.Cluster) (k8s.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if cluster == nil {
		return nil, NotFoundError
	}

	key := apis.Key(cluster)
	if c, ok := c.clients[key]; ok {
		if !timecmp.Equal(cluster.Status.ConnectedAt, c.connectedAt) {
			return nil, StaleClientError
		}

		return c.client, nil
	}

	cli, connectedAt, err := c.provider.GetK8sClient(key)
	if err != nil {
		return nil, err
	}

	c.clients[key] = watermarkedClient{client: cli, connectedAt: connectedAt}
	return cli, nil
}

// Refresh checks to see if there is an updated client for the Cluster object.
//
// If it returns true, any state associated with this Cluster should be reset
// and rebuilt using a new client retrieved via a subsequent call to GetK8sClient.
func (c *ClientManager) Refresh(cluster *v1alpha1.Cluster) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	key := apis.Key(cluster)
	existing, hasExisting := c.clients[key]
	if hasExisting && timecmp.Equal(cluster.Status.ConnectedAt, existing.connectedAt) {
		// we already have a client with the same watermark as passed in
		return false
	}

	// get the canonical version from the provider
	curCli, curConnectedAt, err := c.provider.GetK8sClient(key)
	if !timecmp.Equal(cluster.Status.ConnectedAt, curConnectedAt) {
		// the cluster object we passed in doesn't match the canonical connected
		// at timestamp from the provider, so it must have changed again in the
		// interim, so don't trigger a refresh until the caller is in sync
		return false
	}

	if err != nil {
		// clear the client and return true if it was previously known
		// (we don't care if it changed from one error to another)
		delete(c.clients, key)
		return hasExisting
	}

	// client either previously unknown or config hash changed, refresh!
	c.clients[key] = watermarkedClient{client: curCli, connectedAt: curConnectedAt}
	return true
}
