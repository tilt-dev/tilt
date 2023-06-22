package fake

import (
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func NewFakeTiltClient() fakeTiltClient {
	scheme := v1alpha1.NewScheme()
	cb := fake.NewClientBuilder()
	for _, o := range v1alpha1.AllResourceObjects() {
		cb = cb.WithStatusSubresource(o.(ctrlclient.Object))
	}
	c := cb.WithScheme(scheme).Build()
	return fakeTiltClient{
		Client: c,
	}
}
