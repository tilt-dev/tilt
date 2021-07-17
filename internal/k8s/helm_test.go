package k8s

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"helm.sh/helm/v3/pkg/kube"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// This tests a race condition where if there were two Build()s
// happening in parallel, they would overwrite each other's configs
// due to a race condition.
func TestRESTClientBuilder(t *testing.T) {
	config := &rest.Config{}
	loader, err := clientcmd.NewClientConfigFromBytes(nil)
	require.NoError(t, err)

	client := &K8sClient{
		restConfig:   config,
		clientLoader: loader,
		drm:          fakeRESTMapper{},
	}
	helm := newHelmKubeClient(client)

	g, _ := errgroup.WithContext(context.Background())
	var list1, list2 kube.ResourceList
	g.Go(func() error {
		var err error
		list1, err = helm.Build(strings.NewReader(`
apiVersion: v1
kind: Pod
metadata:
  name: fe
`), false)
		return err
	})

	g.Go(func() error {
		var err error
		list2, err = helm.Build(strings.NewReader(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: fe
`), false)
		return err
	})
	require.NoError(t, g.Wait())

	assert.Equal(t, "v1", list1[0].Client.(*rest.RESTClient).APIVersion().String())
	assert.Equal(t, "apps/v1", list2[0].Client.(*rest.RESTClient).APIVersion().String())
}
