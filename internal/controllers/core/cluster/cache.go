package cluster

import (
	"errors"
	"fmt"
	"sync"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/controllers/apis/cluster"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func NewConnectionManager() *ConnectionManager {
	return &ConnectionManager{}
}

type ConnectionManager struct {
	connections sync.Map
}

var _ cluster.ClientProvider = &ConnectionManager{}

type connectionType string

const (
	connectionTypeK8s    connectionType = "kubernetes"
	connectionTypeDocker connectionType = "docker"
)

type connection struct {
	connType connectionType
	spec     v1alpha1.ClusterSpec

	// createdAt is when the connection object was created.
	// If initError is empty, it's effectively the time we connected to the
	// cluster. Otherwise, it's when we _attempted_ to initialize the client
	// and is used for retry/backoff.
	createdAt time.Time

	// initError is populated when the client cannot be instantiated.
	// For example, if there's no ~/.kube/config, a Kubernetes client
	// can't be created.
	initError string

	dockerClient docker.Client
	k8sClient    k8s.Client

	arch          string
	serverVersion string
	registry      *v1alpha1.RegistryHosting
	connStatus    *v1alpha1.ClusterConnectionStatus
}

func (k *ConnectionManager) GetK8sClient(clusterKey types.NamespacedName) (k8s.Client, metav1.MicroTime, error) {
	conn, err := k.validConnOrError(clusterKey, connectionTypeK8s)
	if err != nil {
		return nil, metav1.MicroTime{}, err
	}
	return conn.k8sClient, apis.NewMicroTime(conn.createdAt), nil
}

// GetComposeDockerClient gets the Docker client for the instance that Docker Compose is deploying to.
//
// This is not currently exposed by the ClientCache interface as Docker Compose logic has not been migrated
// to the apiserver.
func (k *ConnectionManager) GetComposeDockerClient(key types.NamespacedName) (docker.Client, error) {
	conn, err := k.validConnOrError(key, connectionTypeDocker)
	if err != nil {
		return nil, err
	}
	return conn.dockerClient, nil
}

func (k *ConnectionManager) validConnOrError(key types.NamespacedName, connType connectionType) (connection, error) {
	conn, ok := k.load(key)
	if !ok {
		return connection{}, cluster.NotFoundError
	}
	if conn.connType != connType {
		return connection{}, fmt.Errorf("incorrect cluster client type: got %s, expected %s",
			conn.connType, connType)
	}
	if conn.initError != "" {
		return connection{}, errors.New(conn.initError)
	}
	// N.B. even if there is a statusError, the client is still returned, as it
	// might still be functional even though it's in a degraded state
	return conn, nil
}

func (k *ConnectionManager) store(key types.NamespacedName, conn connection) {
	k.connections.Store(key, conn)
}

func (k *ConnectionManager) load(key types.NamespacedName) (connection, bool) {
	v, ok := k.connections.Load(key)
	if !ok {
		return connection{}, false
	}
	return v.(connection), true
}

func (k *ConnectionManager) delete(key types.NamespacedName) {
	k.connections.LoadAndDelete(key)
}
