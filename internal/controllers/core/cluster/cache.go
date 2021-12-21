package cluster

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/controllers/apis/cluster"
	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s"
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
	connType     connectionType
	spec         v1alpha1.ClusterSpec
	dockerClient docker.Client
	k8sClient    k8s.Client
	error        string
	createdAt    time.Time
	arch         string
}

func (k *ConnectionManager) GetK8sClient(key types.NamespacedName) (k8s.Client, time.Time, error) {
	conn, err := k.validConnOrError(key, connectionTypeK8s)
	if err != nil {
		return nil, time.Time{}, err
	}
	return conn.k8sClient, conn.createdAt, nil
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
	if conn.error != "" {
		return connection{}, errors.New(conn.error)
	}
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
	k.connections.Delete(key)
}
