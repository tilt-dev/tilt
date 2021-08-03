package server

import (
	"context"
	"fmt"
	"net"
	"path/filepath"
	"strings"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"github.com/tilt-dev/wmclient/pkg/dirs"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/apiserver"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/options"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/testdata"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"

	"github.com/akutz/memconn"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/openapi"
)

// https://twitter.com/ow/status/1356978380198072321
//
// By default, the API server request limit is 3MB.  Certain Helm Charts with
// CRDs have bigger payloads than this, so we bumped it to 20MB.
//
// (Some custom API servers set it to 100MB, see
// https://github.com/kubernetes/kubernetes/pull/73805)
//
// This doesn't mean large 20MB payloads are fine. Iteratively applying a 20MB
// payload over and over will slow down the overall system, simply on copying
// and encoding/decoding costs alone.
//
// The underlying apiserver libraries have the ability to set this limit on a
// per-resource level (rather than a per-server level). If that's ever exposed,
// we should adjust this limit to be higher for KubernetesApply and lower for
// other resource types.
//
// It also might make sense to help the user break up large payloads.  For
// example, we could automatically split large CRDs into their own resources.
const maxRequestBodyBytes = int64(20 * 1024 * 1024)

type WebListener net.Listener
type APIServerPort int

const storageRelDir = "global"

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
func ProvideTiltServerOptions(
	ctx context.Context,
	tiltBuild model.TiltBuild,
	memconn apiserver.ConnProvider,
	token BearerToken,
	certKey options.GeneratableKeyCert,
	apiPort APIServerPort,
	dir *dirs.TiltDevDir) (*APIServerConfig, error) {
	w := logger.Get(ctx).Writer(logger.DebugLvl)
	builder := builder.NewServerBuilder().
		WithOutputWriter(w).
		WithBearerToken(string(token)).
		WithCertKey(certKey)

	err := dir.MkdirAll(storageRelDir)
	if err != nil {
		return nil, err
	}

	storagePath, err := dir.Abs(storageRelDir)
	if err != nil {
		return nil, err
	}

	for _, obj := range v1alpha1.AllResourceObjects() {
		gvr := obj.GetGroupVersionResource()
		useFileStorage := gvr == (&v1alpha1.ExtensionRepo{}).GetGroupVersionResource() ||
			gvr == (&v1alpha1.GlobalExtension{}).GetGroupVersionResource()
		if useFileStorage {
			builder = builder.WithResourceFileStorage(obj, storagePath)
		} else {
			builder = builder.WithResourceMemoryStorage(obj, "data")
		}
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

	config, err := o.Config()
	if err != nil {
		return nil, err
	}
	config.GenericConfig.MaxRequestBodyBytes = maxRequestBodyBytes
	return config, nil
}

// Generate the server config, removing options that are not needed for testing.
//
// 1) Changes http -> https
// 2) Skips OpenAPI installation
func ProvideTiltServerOptionsForTesting(ctx context.Context, dir *dirs.TiltDevDir) (*APIServerConfig, error) {
	config, err := ProvideTiltServerOptions(ctx,
		model.TiltBuild{}, ProvideMemConn(), "corgi-charge", testdata.CertKey(), 0, dir)
	if err != nil {
		return nil, err
	}

	config.GenericConfig.Config.SkipOpenAPIInstallation = true
	config.GenericConfig.LoopbackClientConfig.TLSClientConfig = rest.TLSClientConfig{}
	config.GenericConfig.LoopbackClientConfig.Host =
		strings.Replace(config.GenericConfig.LoopbackClientConfig.Host, "https://", "http://", 1)
	config.ExtraConfig.ServingInfo.Cert = nil

	return config, nil
}

// Provide a dynamic API client for the Tilt server.
func ProvideTiltDynamic(config *APIServerConfig) (DynamicInterface, error) {
	return dynamic.NewForConfig(config.GenericConfig.LoopbackClientConfig)
}
