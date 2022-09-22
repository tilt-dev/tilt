/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package start

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/spf13/cobra"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/apiserver"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/options"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/authentication/request/anonymous"
	"k8s.io/apiserver/pkg/authorization/authorizerfactory"
	"k8s.io/apiserver/pkg/registry/generic"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/server/dynamiccertificates"
	"k8s.io/apiserver/pkg/storage/storagebackend"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
)

// TiltServerOptions contains state for master/api server
type TiltServerOptions struct {
	scheme               *runtime.Scheme
	codecs               serializer.CodecFactory
	codec                runtime.Codec
	recommendedConfigFns []RecommendedConfigFn
	apis                 map[schema.GroupVersionResource]apiserver.StorageProvider
	ServingOptions       *options.SecureServingOptions
	ConnProvider         apiserver.ConnProvider

	stdout io.Writer
	stderr io.Writer
}

// NewTiltServerOptions returns a new TiltServerOptions
func NewTiltServerOptions(
	out, errOut io.Writer,
	scheme *runtime.Scheme,
	codecs serializer.CodecFactory,
	codec runtime.Codec,
	recommendedConfigFns []RecommendedConfigFn,
	apis map[schema.GroupVersionResource]apiserver.StorageProvider,
	serving *options.SecureServingOptions,
	connProvider apiserver.ConnProvider) *TiltServerOptions {
	// change: apiserver-runtime
	o := &TiltServerOptions{
		scheme:               scheme,
		codecs:               codecs,
		codec:                codec,
		recommendedConfigFns: recommendedConfigFns,
		apis:                 apis,
		ServingOptions:       serving,
		ConnProvider:         connProvider,

		stdout: out,
		stderr: errOut,
	}
	return o
}

// NewCommandStartTiltServer provides a CLI handler for 'start master' command
// with a default TiltServerOptions.
func NewCommandStartTiltServer(defaults *TiltServerOptions, ctx context.Context) *cobra.Command {
	o := *defaults
	cmd := &cobra.Command{
		Short: "Launch a tilt API server",
		Long:  "Launch a tilt API server",
		RunE: func(c *cobra.Command, args []string) error {
			if err := o.Complete(); err != nil {
				return err
			}
			if err := o.Validate(args); err != nil {
				return err
			}
			stoppedCh, err := o.RunTiltServer(c.Context())
			if err != nil {
				return err
			}
			klog.Infof("Serving tilt-apiserver on %s", o.ServingOptions.Listener.Addr())

			<-stoppedCh
			return nil
		},
	}
	cmd.SetContext(ctx)

	flags := cmd.Flags()
	o.ServingOptions.AddFlags(flags)

	return cmd
}

// Validate validates TiltServerOptions
func (o TiltServerOptions) Validate(args []string) error {
	errors := []error{}
	errors = append(errors, o.ServingOptions.Validate()...)
	if o.ServingOptions.BindPort == 0 {
		errors = append(errors, fmt.Errorf("No serve port set"))
	}
	return utilerrors.NewAggregate(errors)
}

// Complete fills in fields required to have valid data
func (o *TiltServerOptions) Complete() error {
	return nil
}

// Config returns config for the api server given TiltServerOptions
func (o *TiltServerOptions) Config() (*apiserver.Config, error) {
	if o.ConnProvider != nil {
		if o.ServingOptions.BindPort == 0 {
			o.ServingOptions.BindPort = 443 // Create a fake port.
		}

		l, err := o.ConnProvider.Listen("tcp", fmt.Sprintf("%s:%d", o.ServingOptions.BindAddress, o.ServingOptions.BindPort))
		if err != nil {
			return nil, err
		}
		o.ServingOptions.Listener = l
	}

	// Check to see if any certificates have been provided.
	// If none have been provided, create one now.
	err := o.ServingOptions.MaybeDefaultWithSelfSignedCerts(
		"localhost", nil, []net.IP{net.ParseIP("127.0.0.1")})
	if err != nil {
		return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	serverConfig := genericapiserver.NewRecommendedConfig(o.codecs)
	serverConfig = o.ApplyRecommendedConfigFns(serverConfig)

	extraConfig := apiserver.ExtraConfig{
		Scheme: o.scheme,
		Codecs: o.codecs,
		APIs:   o.apis,
	}

	err = o.ServingOptions.ApplyTo(&extraConfig.ServingInfo)
	if err != nil {
		return nil, err
	}

	serving := extraConfig.ServingInfo
	if serving == nil || serving.Listener == nil {
		return nil, fmt.Errorf("internal error: no serve config")
	}
	serverConfig.ExternalAddress = serving.Listener.Addr().String()
	serverConfig.Authorization = genericapiserver.AuthorizationInfo{
		Authorizer: authorizerfactory.NewAlwaysDenyAuthorizer(),
	}
	serverConfig.Authentication = genericapiserver.AuthenticationInfo{
		Authenticator: anonymous.NewAuthenticator(),
	}

	cert := extraConfig.ServingInfo.Cert
	if cert == nil {
		return nil, fmt.Errorf("Unable to generate certificates")
	}

	serverConfig.LoopbackClientConfig = o.loopbackClientConfig(cert)
	serverConfig.RESTOptionsGetter = o

	config := &apiserver.Config{
		GenericConfig: serverConfig,
		ExtraConfig:   extraConfig,
	}

	return config, nil
}

func (o TiltServerOptions) loopbackClientConfig(cert dynamiccertificates.CertKeyContentProvider) *rest.Config {
	if o.ServingOptions.BindPort == 0 {
		panic("internal error: LoopbackClientConfig() cannot be calculated before BindPort set")
	}

	caData, _ := cert.CurrentCertKeyContent()

	result := &rest.Config{
		Host:  fmt.Sprintf("https://%s:%d", o.ServingOptions.BindAddress, o.ServingOptions.BindPort),
		QPS:   50,
		Burst: 100,
		TLSClientConfig: rest.TLSClientConfig{
			CAData: caData,
		},
		BearerToken: o.ServingOptions.BearerToken,
	}
	if o.ConnProvider != nil {
		result.Dial = func(ctx context.Context, network, address string) (net.Conn, error) {
			return o.ConnProvider.DialContext(ctx, network, address)
		}
	}
	return result
}

func (o TiltServerOptions) GetRESTOptions(resource schema.GroupResource) (generic.RESTOptions, error) {
	return generic.RESTOptions{
		StorageConfig: &storagebackend.ConfigForResource{
			GroupResource: resource,
			Config: storagebackend.Config{
				Codec: o.codec,
			},
		},
	}, nil
}

// Complete the config and run the Tilt server
func (o TiltServerOptions) RunTiltServer(ctx context.Context) (<-chan struct{}, error) {
	config, err := o.Config()
	if err != nil {
		return nil, err
	}

	return o.RunTiltServerFromConfig(config.Complete(), ctx)
}

// RunTiltServer starts a new TiltServer given TiltServerOptions
func (o TiltServerOptions) RunTiltServerFromConfig(config apiserver.CompletedConfig, ctx context.Context) (<-chan struct{}, error) {
	server, err := config.New()
	if err != nil {
		return nil, err
	}

	server.GenericAPIServer.AddPostStartHookOrDie("start-tilt-server-informers", func(context genericapiserver.PostStartHookContext) error {
		if config.GenericConfig.SharedInformerFactory != nil {
			config.GenericConfig.SharedInformerFactory.Start(context.StopCh)
		}
		return nil
	})

	prepared := server.GenericAPIServer.PrepareRun()
	serving := config.ExtraConfig.ServingInfo

	tlsConfig, err := TLSConfig(ctx, serving)
	if err != nil {
		return nil, err
	}

	stopCh := ctx.Done()
	stoppedCh, _, err := genericapiserver.RunServer(&http.Server{
		Addr:           serving.Listener.Addr().String(),
		Handler:        prepared.Handler,
		MaxHeaderBytes: 1 << 20,
		TLSConfig:      tlsConfig,
	}, serving.Listener, prepared.ShutdownTimeout, stopCh)
	if err != nil {
		return nil, err
	}

	server.GenericAPIServer.RunPostStartHooks(stopCh)

	return stoppedCh, nil
}
