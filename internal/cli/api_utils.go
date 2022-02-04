package cli

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

func newClient(ctx context.Context) (client.Client, error) {
	getter, err := wireClientGetter(ctx)
	if err != nil {
		return nil, err
	}

	cfg, err := getter.ToRESTConfig()
	if err != nil {
		return nil, err
	}

	ctrlclient, err := client.New(cfg, client.Options{Scheme: v1alpha1.NewScheme()})
	if err != nil {
		return nil, err
	}

	return ctrlclient, err
}
