package fake

import (
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"

	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func NewFakeTiltClient() ctrlclient.Client {
	scheme := v1alpha1.NewScheme()
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		Build()
	return fakeTiltClient{
		Client: c,
	}
}
