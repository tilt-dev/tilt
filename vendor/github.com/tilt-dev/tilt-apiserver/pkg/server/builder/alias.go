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
	"github.com/spf13/cobra"
	builderrest "github.com/tilt-dev/tilt-apiserver/pkg/server/builder/rest"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/start"
	"k8s.io/apiserver/pkg/registry/rest"
	pkgserver "k8s.io/apiserver/pkg/server"
	"k8s.io/kube-openapi/pkg/common"
)

// GenericAPIServer is an alias for pkgserver.GenericAPIServer
type GenericAPIServer = pkgserver.GenericAPIServer

// ServerOptions is an alias for server.ServerOptions
type ServerOptions = start.ServerOptions

// OpenAPIDefinition is an alias for common.OpenAPIDefinition
type OpenAPIDefinition = common.OpenAPIDefinition

// Storage is an alias for rest.Storage.  Storage implements the interfaces defined in the rest package
// to expose new REST endpoints for a Kubernetes resource.
type Storage = rest.Storage

// Command is an alias for cobra.Command and is used to start the apiserver.
type Command = cobra.Command

// DefaultStrategy is a default strategy that may be embedded into other strategies
type DefaultStrategy = builderrest.DefaultStrategy
