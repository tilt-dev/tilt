/*
Copyright 2020 The Kubernetes Authors.

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

package builder

import (
	"flag"
	"io"
	"net"
	"os"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/apiserver"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/start"
	"github.com/tilt-dev/tilt-apiserver/pkg/storage/filepath"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	genericapiserver "k8s.io/apiserver/pkg/server"
	genericoptions "k8s.io/apiserver/pkg/server/options"
)

// NewServerBuilder builds an apiserver to server Kubernetes resources and sub resources.
func NewServerBuilder() *Server {
	scheme := apiserver.NewScheme()
	return &Server{
		stdout:  os.Stdout,
		stderr:  os.Stderr,
		scheme:  scheme,
		codecs:  serializer.NewCodecFactory(scheme),
		storage: map[schema.GroupResource]*singletonProvider{},
		apis:    map[schema.GroupVersionResource]apiserver.StorageProvider{},
		serving: &genericoptions.DeprecatedInsecureServingOptions{
			BindAddress: net.ParseIP("127.0.0.1"),
		},
	}
}

// Server builds a new apiserver for a single API group
type Server struct {
	stdout               io.Writer
	stderr               io.Writer
	scheme               *runtime.Scheme
	codecs               serializer.CodecFactory
	recommendedConfigFns []start.RecommendedConfigFn
	apis                 map[schema.GroupVersionResource]apiserver.StorageProvider
	memoryFS             *filepath.MemoryFS
	errs                 []error
	storage              map[schema.GroupResource]*singletonProvider
	groupVersions        map[schema.GroupVersion]bool
	orderedGroupVersions []schema.GroupVersion
	schemeBuilder        runtime.SchemeBuilder
	serving              *genericoptions.DeprecatedInsecureServingOptions
	connProvider         apiserver.ConnProvider
}

func (a *Server) buildCodec() (runtime.Codec, error) {
	a.schemeBuilder.Register(
		func(scheme *runtime.Scheme) error {
			groupVersions := make(map[string]sets.String)
			for gvr := range a.apis {
				if groupVersions[gvr.Group] == nil {
					groupVersions[gvr.Group] = sets.NewString()
				}
				groupVersions[gvr.Group].Insert(gvr.Version)
			}
			for g, versions := range groupVersions {
				gvs := []schema.GroupVersion{}
				for _, v := range versions.List() {
					gvs = append(gvs, schema.GroupVersion{
						Group:   g,
						Version: v,
					})
				}
				err := scheme.SetVersionPriority(gvs...)
				if err != nil {
					return err
				}
			}
			for i := range a.orderedGroupVersions {
				metav1.AddToGroupVersion(scheme, a.orderedGroupVersions[i])
			}
			return nil
		},
	)
	if err := a.schemeBuilder.AddToScheme(a.scheme); err != nil {
		panic(err)
	}

	if len(a.errs) != 0 {
		return nil, errs{list: a.errs}
	}

	return a.codecs.LegacyCodec(a.orderedGroupVersions...), nil
}

// Builds a server options interpreter that can run the server.
// Intended when calling the server programatically.
func (a *Server) ToServerOptions() (*start.TiltServerOptions, error) {
	codec, err := a.buildCodec()
	if err != nil {
		return nil, err
	}
	return start.NewTiltServerOptions(a.stdout, a.stderr, a.scheme,
		a.codecs, codec, a.recommendedConfigFns, a.apis, a.serving, a.connProvider), nil
}

// Builds a cobra command that runs the server.
// Intended when connecting the server to a CLI.
func (a *Server) ToServerCommand() (*Command, error) {
	codec, err := a.buildCodec()
	if err != nil {
		return nil, err
	}

	o := start.NewTiltServerOptions(a.stdout, a.stderr, a.scheme,
		a.codecs, codec, a.recommendedConfigFns, a.apis, a.serving, a.connProvider)
	cmd := start.NewCommandStartTiltServer(o, genericapiserver.SetupSignalHandler())
	cmd.Flags().AddGoFlagSet(flag.CommandLine)
	return cmd, nil
}

// Execute builds and executes the apiserver Command.
func (a *Server) ExecuteCommand() error {
	cmd, err := a.ToServerCommand()
	if err != nil {
		return err
	}

	return cmd.Execute()
}
