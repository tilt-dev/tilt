package cluster

import (
	"context"

	"github.com/tilt-dev/tilt/internal/docker"
	"github.com/tilt-dev/tilt/internal/k8s"
)

type KubernetesClientFactory interface {
	New(ctx context.Context, contextOverride k8s.KubeContextOverride, namespaceOverride k8s.NamespaceOverride) (k8s.Client, error)
}

type KubernetesClientFunc func(ctx context.Context, contextOverride k8s.KubeContextOverride, namespaceOverride k8s.NamespaceOverride) (k8s.Client, error)

func (k KubernetesClientFunc) New(ctx context.Context, contextOverride k8s.KubeContextOverride, namespaceOverride k8s.NamespaceOverride) (k8s.Client, error) {
	return k(ctx, contextOverride, namespaceOverride)
}

type DockerClientFactory interface {
	New(ctx context.Context, env docker.Env) (docker.Client, error)
}

type DockerClientFunc func(ctx context.Context, env docker.Env) (docker.Client, error)

func (d DockerClientFunc) New(ctx context.Context, env docker.Env) (docker.Client, error) {
	return d(ctx, env)
}

func DockerClientFromEnv(ctx context.Context, env docker.Env) (docker.Client, error) {
	client := docker.NewDockerClient(ctx, env)
	err := client.CheckConnected()
	if err != nil {
		return nil, err
	}
	return client, nil
}

// KubernetesClientFromEnv creates a client based on the machine environment.
//
// The Kubernetes Client APIs are really defined for automatic dependency injection.
// (as opposed to the Kubernetes convention of nested factory structs.)
//
// If you have to edit the below, it's easier to let wire generate the
// factory code for you, then adapt it here.
func KubernetesClientFromEnv(ctx context.Context, contextOverride k8s.KubeContextOverride, namespaceOverride k8s.NamespaceOverride) (k8s.Client, error) {
	clientConfig := k8s.ProvideClientConfig(contextOverride, namespaceOverride)
	apiConfigOrError := k8s.ProvideAPIConfig(clientConfig, contextOverride, namespaceOverride)
	if apiConfigOrError.Error != nil {
		return nil, apiConfigOrError.Error
	}
	env := k8s.ProvideClusterProduct(apiConfigOrError)
	restConfigOrError := k8s.ProvideRESTConfig(clientConfig)

	clientsetOrError := k8s.ProvideClientset(restConfigOrError)
	portForwardClient := k8s.ProvidePortForwardClient(restConfigOrError, clientsetOrError)
	namespace := k8s.ProvideConfigNamespace(clientConfig)
	kubeContext := k8s.ProvideKubeContext(apiConfigOrError)
	clusterName := k8s.ProvideClusterName(apiConfigOrError)
	minikubeClient := k8s.ProvideMinikubeClient(clusterName)
	client := k8s.ProvideK8sClient(ctx, env, restConfigOrError, clientsetOrError, portForwardClient, kubeContext, clusterName, namespace, minikubeClient, apiConfigOrError, clientConfig)
	_, err := client.CheckConnected(ctx)
	if err != nil {
		return nil, err
	}
	return client, nil
}
