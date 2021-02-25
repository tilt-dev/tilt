package server

import (
	"context"
	"fmt"
	"net"

	"k8s.io/client-go/dynamic"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/apiserver"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder"
	"github.com/tilt-dev/tilt/pkg/clientset/tiltapi"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/akutz/memconn"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/openapi"
)

type APIServerConfig = apiserver.Config

type DynamicInterface = dynamic.Interface
type Interface = tiltapi.Interface

func ProvideMemConn() *memconn.Provider {
	return &memconn.Provider{}
}

// Configures the Tilt API server.
func ProvideTiltServerOptions(ctx context.Context, host model.WebHost, port model.WebPort, tiltBuild model.TiltBuild, memconn *memconn.Provider) (*APIServerConfig, error) {
	w := logger.Get(ctx).Writer(logger.DebugLvl)
	builder := builder.NewServerBuilder().
		WithOutputWriter(w)

	for _, obj := range v1alpha1.AllResourceObjects() {
		builder = builder.WithResourceMemoryStorage(obj, "data")
	}
	builder = builder.WithOpenAPIDefinitions("tilt", tiltBuild.Version, openapi.GetOpenAPIDefinitions)
	if port == 0 {
		builder = builder.WithConnProvider(memconn)
	} else {
		builder = builder.WithBindPort(int(port))
	}

	o, err := builder.ToServerOptions()
	if err != nil {
		return nil, err
	}

	if port == 0 {
		// Fake bind port
		o.ServingOptions.BindPort = 1
	} else {
		l, err := net.Listen("tcp", fmt.Sprintf("%s:%d", string(host), int(port)))
		if err != nil {
			return nil, fmt.Errorf("Tilt cannot start because you already have another process on port %d\n"+
				"If you want to run multiple Tilt instances simultaneously,\n"+
				"use the --port flag or TILT_PORT env variable to set a custom port\nOriginal error: %v",
				port, err)
		}
		o.ServingOptions.Listener = l
	}

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

// Provide a typed API client for the Tilt server.
func ProvideTiltInterface(config *APIServerConfig) (Interface, error) {
	return tiltapi.NewForConfig(config.GenericConfig.LoopbackClientConfig)
}

// Provide a dynamic API client for the Tilt server.
func ProvideTiltDynamic(config *APIServerConfig) (DynamicInterface, error) {
	return dynamic.NewForConfig(config.GenericConfig.LoopbackClientConfig)
}
