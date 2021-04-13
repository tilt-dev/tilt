package fake

import (
	"k8s.io/client-go/rest"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
)

func NewTiltClient() ctrlclient.Client {
	scheme := v1alpha1.NewScheme()
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()
	return fakeTiltClient{
		Client: c,
	}
}

func NewClientBuilder(client ctrlclient.Client) *fakeClientBuilder {
	return &fakeClientBuilder{client: client}
}

// A stub builder that returns a pre-populated client.
type fakeClientBuilder struct {
	client ctrlclient.Client
}

func (b *fakeClientBuilder) WithUncached(objs ...client.Object) cluster.ClientBuilder {
	return b
}

func (b *fakeClientBuilder) Build(cache cache.Cache, config *rest.Config, options client.Options) (client.Client, error) {
	return b.client, nil
}
