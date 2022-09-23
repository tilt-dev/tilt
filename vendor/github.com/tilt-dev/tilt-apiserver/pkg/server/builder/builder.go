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
	"github.com/tilt-dev/tilt-apiserver/pkg/server/options"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/start"
	"github.com/tilt-dev/tilt-apiserver/pkg/storage/filepath"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	genericapiserver "k8s.io/apiserver/pkg/server"
)

// NewServerBuilder builds an apiserver to server Kubernetes resources and sub resources.
func NewServerBuilder() *Server {

	// In the "real" Kubernetes server, every type has:
	//
	// 1) An internal version (a superset of all fields across all versions)
	// 2) A storage version (how the object is stored)
	// 3) Multiple API versions to expose on the apiserver.
	//
	// And then we do a Conversion from
	// API Version -> Internal Version -> Storage Version.
	//
	// (The Patch() API hard-codes the internal version to do patch updates.)
	//
	// Here's a good post on this:
	// https://cloud.redhat.com/blog/kubernetes-deep-dive-api-server-part-2
	//
	// In our simplified Server Builder, we only have one version registered
	// as both Internal and Storage version.
	//
	// This MOSTLY works, but causes problems for the openapi generator,
	// because it gets confused that the same reflect.Type is registered twice.
	//
	// To hack around this, we use a separate scheme for openapi generation.
	apiScheme := apiserver.NewScheme()
	openapiScheme := apiserver.NewScheme()

	return &Server{
		stdout:        os.Stdout,
		stderr:        os.Stderr,
		apiScheme:     apiScheme,
		openapiScheme: openapiScheme,
		codecs:        serializer.NewCodecFactory(apiScheme),
		storage:       map[schema.GroupResource]*singletonProvider{},
		apis:          map[schema.GroupVersionResource]apiserver.StorageProvider{},
		serving: &options.SecureServingOptions{
			BindAddress: net.ParseIP("127.0.0.1"),
		},
	}
}

// Server builds a new apiserver for a single API group
type Server struct {
	stdout io.Writer
	stderr io.Writer

	apiScheme            *runtime.Scheme
	openapiScheme        *runtime.Scheme
	apiSchemeBuilder     runtime.SchemeBuilder
	openapiSchemeBuilder runtime.SchemeBuilder

	codecs               serializer.CodecFactory
	recommendedConfigFns []start.RecommendedConfigFn
	apis                 map[schema.GroupVersionResource]apiserver.StorageProvider
	memoryFS             *filepath.MemoryFS
	errs                 []error
	storage              map[schema.GroupResource]*singletonProvider
	groupVersions        map[schema.GroupVersion]bool
	orderedGroupVersions []schema.GroupVersion
	serving              *options.SecureServingOptions
	connProvider         apiserver.ConnProvider
}

func (a *Server) buildCodec() (runtime.Codec, error) {
	registerGroupVersions := func(scheme *runtime.Scheme) error {
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
	}
	a.apiSchemeBuilder.Register(registerGroupVersions)
	if err := a.apiSchemeBuilder.AddToScheme(a.apiScheme); err != nil {
		return nil, err
	}
	a.openapiSchemeBuilder.Register(registerGroupVersions)
	if err := a.openapiSchemeBuilder.AddToScheme(a.openapiScheme); err != nil {
		return nil, err
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
	return start.NewTiltServerOptions(a.stdout, a.stderr, a.apiScheme,
		a.codecs, codec, a.recommendedConfigFns, a.apis, a.serving, a.connProvider), nil
}

// Builds a cobra command that runs the server.
// Intended when connecting the server to a CLI.
func (a *Server) ToServerCommand() (*Command, error) {
	codec, err := a.buildCodec()
	if err != nil {
		return nil, err
	}

	o := start.NewTiltServerOptions(a.stdout, a.stderr, a.apiScheme,
		a.codecs, codec, a.recommendedConfigFns, a.apis, a.serving, a.connProvider)
	cmd := start.NewCommandStartTiltServer(o, genericapiserver.SetupSignalContext())
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
