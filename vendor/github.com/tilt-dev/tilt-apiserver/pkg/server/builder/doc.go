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

// Package builder contains a builder for creating a new Kubernetes apiserver.
//
// API Extension Servers
//
// API extension servers and apiserver aggregation are techniques for extending the Kubernetes API surface
// without using CRDs.  Rather than registering a resource type as a CRD stored by the apiserver in etcd, apiserver
// aggregation registers REST endpoints provided by the extension server, and requests are proxied by the main
// control-plane apiserver to the extension apiserver.
//
// Use Cases
//
// Following are use cases where one may consider using an extension API server rather than CRDs for implementing
// an extension resource type.
//
// * Resource types which are not backed by storage -- e.g. metrics
//
// * Resource types which may not fit in etcd
//
// * Using a separate etcd instance for the extension types
//
// Registering Types
//
// New resource types may be registered with the API server by implementing the go struct for the type under
// YOUR_MODULE/pkg/apis/YOUR_GROUP/VERSION/types.go and then calling WithResource.
// You will need to generate deepcopy and openapi go code for your types to be registered.
//
// Install the code generators (from your module):
//
//    $ go get sigs.k8s.io/apiserver-runtime/tools/apiserver-runtime-gen
//    $ apiserver-runtime-gen --install
//
// Add the code generation tag to you main package:
//
//    //go:generate apiserver-runtime-gen
//    package main
//
// Run the code generation after having defined your types:
//
//    $ go generate ./...
//
// To also generate clients, provide the -g option to apiserver-runtime-gen for the client, lister and informer
// generators.
//
//    $ apiserver-runtime-gen -g client-gen -g deepcopy-gen -g informer-gen -g lister-gen -g openapi-gen
//
// Implementing Type Specific Logic
//
// * How an object is stored may be customized by either 1) implementing interfaces defined in
// pkg/builder/resource/resourcestrategy or 2) providing a Strategy when registering the type with the builder.
//
// * How a request is handled may be customized by either 1) implementing the interfaces defined in
// pkg/builder/resource/resourcerest or 2) providing a HandlerProvider when registering the type with the builder.
//
// If the go struct for the resource type implements the resource interfaces, they will automatically be used
// when the resource type is registered with the builder.
package builder
