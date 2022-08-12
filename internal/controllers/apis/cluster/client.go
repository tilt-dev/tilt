package cluster

import (
	"errors"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
var StaleClientError = errors.New("cluster revision is stale")

// ClientProvider provides client instances to the ClientManager.
//
// All clients MUST be goroutine-safe.
type ClientProvider interface {
	// GetK8sClient returns the Kubernetes client for the cluster or an error for unknown clusters, connections
	// in a transient error state, or if the connection is of a different type (i.e. Docker Compose).
	//
	// In addition to the client, the timestamp at which the client was created is returned so callers can track
	// when the client instance has changed.
	GetK8sClient(clusterKey types.NamespacedName) (k8s.Client, metav1.MicroTime, error)
}

type clusterRef struct {
	objKey     types.NamespacedName
	clusterKey types.NamespacedName
}

type clientRevision struct {
	connectedAt metav1.MicroTime
	client      k8s.Client
}

// ClientManager is a convenience wrapper over ClientProvider which simplifies
// handling client changes.
//
// On reconcile, the controller should:
//
//	(1) Fetch the Cluster object referenced by the type its reconciling.
//	(2) Call ClientManager::Refresh to determine if the client for the cluster
//		has changed. If true, all state associated with the old cluster should
//		be cleared.
//	(3) As needed, call ClientManager::GetK8sClient to get a client instance.
type ClientManager struct {
	mu       sync.Mutex
	provider ClientProvider

	revisions map[clusterRef]metav1.MicroTime
}

func NewClientManager(clientProvider ClientProvider) *ClientManager {
	return &ClientManager{
		provider:  clientProvider,
		revisions: make(map[clusterRef]metav1.MicroTime),
	}
}

// GetK8sClient returns the client associated with the Cluster object.
//
// If no client is known for the Cluster object, NotFoundError is returned.
// If the Cluster object's config hash in the status does not match the known client, StaleClientError is returned.
func (c *ClientManager) GetK8sClient(obj apis.KeyableObject, cluster *v1alpha1.Cluster) (k8s.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.getK8sClient(obj, cluster)
}

// Refresh checks to see if there is an updated client for the Cluster object.
//
// If it returns true, any state associated with this Cluster should be reset
// and rebuilt using a new client retrieved via a subsequent call to GetK8sClient.
func (c *ClientManager) Refresh(obj apis.KeyableObject, cluster *v1alpha1.Cluster) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	clusterRef := clusterRef{clusterKey: apis.Key(cluster), objKey: apis.Key(obj)}
	objRevision, knownForObj := c.revisions[clusterRef]

	if knownForObj && timecmp.Equal(cluster.Status.ConnectedAt, objRevision) {
		// regardless of if there's a new client, state from the perspective
		// of this caller is in sync because the client its tracking matches
		// the cluster object it passed in, so having it potentially reset state
		// (assuming an updated client exists) will be more harmful than helpful
		// as it won't be able to fetch it
		return false
	}

	_, err := c.getK8sClient(obj, cluster)
	if err != nil {
		delete(c.revisions, clusterRef)
		return knownForObj
	}

	return false
}

func (c *ClientManager) getK8sClient(obj apis.KeyableObject, cluster *v1alpha1.Cluster) (k8s.Client, error) {
	if cluster == nil {
		return nil, NotFoundError
	}

	clusterNN := apis.Key(cluster)
	cli, revision, err := c.provider.GetK8sClient(clusterNN)
	if err != nil {
		return nil, err
	}

	if !timecmp.Equal(cluster.Status.ConnectedAt, revision) {
		// the client does not match the cluster object that was passed in
		return nil, StaleClientError
	}

	clusterRef := clusterRef{objKey: apis.Key(obj), clusterKey: clusterNN}
	if objRevision, ok := c.revisions[clusterRef]; ok {
		if !timecmp.Equal(revision, objRevision) {
			// client previously had an old version of client and need to call
			// refresh to clear out their old state
			return nil, StaleClientError
		}
	} else {
		// first time this object has fetched this client, so track the version
		// we gave it so we can detect when it becomes stale
		c.revisions[clusterRef] = revision
	}

	return cli, nil
}
