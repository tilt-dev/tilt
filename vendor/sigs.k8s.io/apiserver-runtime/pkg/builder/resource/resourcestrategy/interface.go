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

package resourcestrategy

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// AllowCreateOnUpdater is invoked by the DefaultStrategy
type AllowCreateOnUpdater interface {
	// AllowCreateOnUpdate is invoked by the DefaultStrategy
	AllowCreateOnUpdate() bool
}

// AllowUnconditionalUpdater is invoked by the DefaultStrategy
type AllowUnconditionalUpdater interface {
	// AllowUnconditionalUpdate is invoked by the DefaultStrategy
	AllowUnconditionalUpdate() bool
}

// Canonicalizer functions are invoked before an object is stored to canonicalize the object's format.
// If Canonicalize is implemented fr a type, it will be invoked before storing an object of that type for
// either a create or update.
//
// Canonicalize is only invoked for the type that is the storage version type.
type Canonicalizer interface {
	// Canonicalize formats the object for storage.  Only applied for the version matching the storage version.
	Canonicalize()
}

// Converter defines functions for converting a version of a resource to / from the internal version.
// Converter functions are called to convert the request version of the object to the handler version --
// e.g. if a v1beta1 object is created, and the handler uses a v1alpha1 version, then the v1beta1 will be converted
// to a v1alpha1 before the handler is called.
type Converter interface {
	// ConvertFromInternal converts an internal version of the object to this object's version
	ConvertFromInternal(internal interface{})

	// ConvertToInternal converts this version of the object to an internal version of the object.
	ConvertToInternal() (internal interface{})
}

// Defaulter functions are invoked when deserializing an object.  If Default is implemented for a type, the apiserver
// will use it to perform defaulting for that version before converting it to the handler version.
// Different versions of a resource may have different Defaulter implementations.
type Defaulter interface {
	// Default defaults unset values on the object.  Defaults are specific to the version.
	Default()
}

// PrepareForCreater functions are invoked before an object is stored during creation.  If PrepareForCreate
// is implemented for a type, it will be invoked before creating an object of that type.
//
// PrepareForCreater is only invoked when storing an object and only for the type that is the storage version type.
type PrepareForCreater interface {
	PrepareForCreate(ctx context.Context)
}

// PrepareForUpdater functions are invoked before an object is stored during update.  If PrepareForCreate
// is implemented for a type, it will be invoked before updating an object of that type.
//
// PrepareForUpdater is only invoked when storing an object and only for the type that is the storage version type.
type PrepareForUpdater interface {
	PrepareForUpdate(ctx context.Context, old runtime.Object)
}

// TableConverter functions are invoked when printing an object from `kubectl get`.
type TableConverter interface {
	ConvertToTable(ctx context.Context, tableOptions runtime.Object) (*metav1.Table, error)
}

// Validater functions are invoked before an object is stored to validate the object during creation.  If Validate
// is implemented for a type, it will be invoked before creating an object of that type.
type Validater interface {
	Validate(ctx context.Context) field.ErrorList
}

// ValidateUpdater functions are invoked before an object is stored to validate the object during update.
// If ValidateUpdater is implemented for a type, it will be invoked before updating an object of that type.
type ValidateUpdater interface {
	ValidateUpdate(ctx context.Context, obj runtime.Object) field.ErrorList
}
