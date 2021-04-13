package server

import (
	"context"
	"fmt"
	"net"
	"path/filepath"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/tilt-dev/wmclient/pkg/dirs"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/apiserver"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/options"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/akutz/memconn"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/openapi"
)

type WebListener net.Listener
type APIServerPort int

type APIServerConfig = apiserver.Config

type DynamicInterface = dynamic.Interface

func ProvideMemConn() apiserver.ConnProvider {
	return apiserver.NetworkConnProvider(&memconn.Provider{}, "memu")
}

func ProvideKeyCert() options.GeneratableKeyCert {
	return options.GeneratableKeyCert{}
}

// Uses the kubernetes config-loading library to load
// api configs from disk.
//
// Usually loads from ~/.windmill/config or ~/tilt-dev/config.
//
// Also allows overriding with the TILT_CONFIG env variable, like
// TILT_CONFIG=./path/to/my/config
// which is useful when testing CLIs.
func ProvideConfigAccess(dir *dirs.TiltDevDir) clientcmd.ConfigAccess {
	ret := &clientcmd.PathOptions{
		GlobalFile:        filepath.Join(dir.Root(), "config"),
		GlobalFileSubpath: filepath.Join(filepath.Dir(dir.Root()), "config"),
		EnvVar:            "TILT_CONFIG",
		LoadingRules:      clientcmd.NewDefaultClientConfigLoadingRules(),
	}
	ret.LoadingRules.DoNotResolvePaths = true
	return ret
}

// Creates a listener for the plain http web server.
func ProvideWebListener(host model.WebHost, port model.WebPort) (WebListener, error) {
	webListener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", string(host), int(port)))
	if err != nil {
		return nil, fmt.Errorf("Tilt cannot start because you already have another process on port %d\n"+
			"If you want to run multiple Tilt instances simultaneously,\n"+
			"use the --port flag or TILT_PORT env variable to set a custom port\nOriginal error: %v",
			port, err)
	}
	return WebListener(webListener), nil
}

// Picks a random port for the APIServer.
//
// TODO(nick): In the future, we should be able to have the apiserver listen
// on other network interfaces, not just loopback. But then we would have to
// also setup the KeyCert to identify the server.
func ProvideAPIServerPort() (APIServerPort, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return APIServerPort(l.Addr().(*net.TCPAddr).Port), nil
}

// Configures the Tilt API server.
func ProvideTiltServerOptions(ctx context.Context,
	tiltBuild model.TiltBuild,
	memconn apiserver.ConnProvider,
	token BearerToken,
	certKey options.GeneratableKeyCert,
	apiPort APIServerPort) (*APIServerConfig, error) {
	w := logger.Get(ctx).Writer(logger.DebugLvl)
	builder := builder.NewServerBuilder().
		WithOutputWriter(w).
		WithBearerToken(string(token)).
		WithCertKey(certKey)

	for _, obj := range v1alpha1.AllResourceObjects() {
		builder = builder.WithResourceMemoryStorage(obj, "data")
	}
	builder = builder.WithOpenAPIDefinitions("tilt", tiltBuild.Version, openapi.GetOpenAPIDefinitions)

	if apiPort == 0 {
		// If no API port is provided, that means we're in test mode and should use
		// in-memory connections.
		builder = builder.WithConnProvider(memconn)
	} else {
		builder = builder.WithBindPort(int(apiPort))
	}

	o, err := builder.ToServerOptions()
	if err != nil {
		return nil, err
	}

	if apiPort == 0 {
		// Fake bind port
		o.ServingOptions.BindPort = 1
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

// Provide a dynamic API client for the Tilt server.
func ProvideTiltDynamic(config *APIServerConfig) (DynamicInterface, error) {
	return dynamic.NewForConfig(config.GenericConfig.LoopbackClientConfig)
}
