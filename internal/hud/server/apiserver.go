package server

import (
	"context"

	"k8s.io/client-go/dynamic"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/apiserver"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/akutz/memconn"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/openapi"
)

type APIServerConfig = apiserver.Config

type DynamicInterface = dynamic.Interface

func ProvideMemConn() apiserver.ConnProvider {
	return apiserver.NetworkConnProvider(&memconn.Provider{}, "memu")
}

// Configures the Tilt API server.
func ProvideTiltServerOptions(ctx context.Context, tiltBuild model.TiltBuild, memconn apiserver.ConnProvider) (*APIServerConfig, error) {
	w := logger.Get(ctx).Writer(logger.DebugLvl)
	builder := builder.NewServerBuilder().
		WithOutputWriter(w)

	for _, obj := range v1alpha1.AllResourceObjects() {
		builder = builder.WithResourceMemoryStorage(obj, "data")
	}
	builder = builder.WithOpenAPIDefinitions("tilt", tiltBuild.Version, openapi.GetOpenAPIDefinitions)

	// TODO(nick): We're going to split the APIServer into a separate port
	// that speaks HTTPS instead of HTTP. For the transition period,
	// we'll use the in-memory connection.
	builder = builder.WithConnProvider(memconn)

	o, err := builder.ToServerOptions()
	if err != nil {
		return nil, err
	}

	// Fake bind port
	o.ServingOptions.BindPort = 1

	err = o.Complete()
	if err != nil {
		return nil, err
	}
	err = o.Validate(nil)
	if err != nil {
		return nil, err
	}
	return o.Config()
}

// Provide a dynamic API client for the Tilt server.
func ProvideTiltDynamic(config *APIServerConfig) (DynamicInterface, error) {
	return dynamic.NewForConfig(config.GenericConfig.LoopbackClientConfig)
}
