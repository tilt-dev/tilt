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

package resourcerest

import "k8s.io/apiserver/pkg/registry/rest"

// CategoriesProvider allows a resource to specify which groups of resources (categories) it's part of. Categories can
// be used by API clients to refer to a batch of resources by using a single name (e.g. "all" could translate to "pod,rc,svc,...").
type CategoriesProvider = rest.CategoriesProvider

// Creator if implemented will expose PUT endpoints for the resource and publish them in the Kubernetes
// discovery service and OpenAPI.
//
// Required for `kubectl apply`.
type Creator = rest.Creater

// CollectionDeleter if implemented will expose DELETE endpoints for resource collections and publish them in
// the Kubernetes discovery service and OpenAPI.
//
// Required for `kubectl delete --all`
type CollectionDeleter = rest.CollectionDeleter

// Connecter is a storage object that responds to a connection request.
type Connecter = rest.Connecter

// CreaterUpdater is a storage object that must support both create and update.
// Go prevents embedded interfaces that implement the same method.
type CreaterUpdater = rest.CreaterUpdater

// Getter if implemented will expose GET endpoints for the resource and publish them in the Kubernetes
// discovery service and OpenAPI.
//
// Required for `kubectl apply` and most operators.
type Getter = rest.Getter

// GracefulDeleter knows how to pass deletion options to allow delayed deletion of a
// RESTful object.
type GracefulDeleter = rest.GracefulDeleter

// Lister if implemented will enable listing resources.
//
// Required by `kubectl get` and most operators.
type Lister = rest.Lister

// Patcher if implemented will expose POST and GET endpoints for the resource and publish them in the Kubernetes
// discovery service and OpenAPI.
//
// Required by `kubectl apply` and most controllers.
type Patcher = rest.Patcher

// Redirector know how to return a remote resource's location.
type Redirector = rest.Redirector

// Responder abstracts the normal response behavior for a REST method and is passed to callers that
// may wish to handle the response directly in some cases, but delegate to the normal error or object
// behavior in other cases.
type Responder = rest.Responder

// ShortNamesProvider is an interface for RESTful storage services. Delivers a list of short
// names for a resource. The list is used by kubectl to have short names representation of resources.
type ShortNamesProvider = rest.ShortNamesProvider

// TableConvertor if implemented will return tabular data from the GET endpoint when requested.
//
// Required by pretty printing `kubectl get`.
type TableConvertor = rest.TableConvertor

// Updater if implemented will expose POST endpoints for the resource and publish them in the Kubernetes
// discovery service and OpenAPI.
//
// Required by `kubectl apply` and most controllers.
type Updater = rest.Updater

// Watcher if implemented will enable watching resources.
//
// Required by most controllers.
type Watcher = rest.Watcher

// StandardStorage defines the standard endpoints for resources.
type StandardStorage = rest.StandardStorage

// FieldsIndexer indices resources by certain fields at the server-side.
// TODO: implement it
type FieldsIndexer interface {
	IndexingFields() []string
	GetField(fieldName string) string
}

// LabelsIndexer indices resources by their labels at the server-side.
// TODO: implement it
type LabelsIndexer interface {
	IndexingLabelKeys() []string
}
