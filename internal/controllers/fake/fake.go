package fake

import (
	"k8s.io/client-go/rest"

	"github.com/tilt-dev/tilt/internal/controllers"

	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/cluster"
)

func NewTiltClient() ctrlclient.Client {
	scheme := controllers.NewScheme()
	return fake.NewClientBuilder().
		WithScheme(scheme).
		Build()
}

func NewClientBuilder(client ctrlclient.Client) *fakeClientBuilder {
	return &fakeClientBuilder{client: client}
}

// A stub builder that returns a pre-poplated client.
type fakeClientBuilder struct {
	client ctrlclient.Client
}

func (b *fakeClientBuilder) WithUncached(objs ...client.Object) cluster.ClientBuilder {
	return b
}

func (b *fakeClientBuilder) Build(cache cache.Cache, config *rest.Config, options client.Options) (client.Client, error) {
	return b.client, nil
}
