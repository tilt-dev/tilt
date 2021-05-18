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

// Package resourcerest defines interfaces for resource REST implementations.
//
// If a resource implements these interfaces directly on the object, then the resource itself may be used
// as the request handler, and will be registered as the REST handler by default when
// builder.APIServer.WithResource is called.
//
// Alternatively, a REST struct may be defined separately from the object and explicitly registered to handle the
// object with builder.APIServer.WithResourceAndHandler.
package resourcerest
