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

package rest

import (
	"context"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource/resourcestrategy"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"
)

// Strategy defines functions that are invoked prior to storing a Kubernetes resource.
type Strategy interface {
	WarningsOnCreate(ctx context.Context, obj runtime.Object) []string
	AllowCreateOnUpdate() bool
	AllowUnconditionalUpdate() bool
	Match(label labels.Selector, field fields.Selector) storage.SelectionPredicate
	rest.RESTUpdateStrategy
	rest.RESTCreateStrategy
	rest.RESTDeleteStrategy
	rest.TableConvertor
	rest.ShortNamesProvider
	rest.SingularNameProvider
}

var _ Strategy = DefaultStrategy{}

// DefaultStrategy implements Strategy.  DefaultStrategy may be embedded in another struct to override
// is implementation.  DefaultStrategy will delegate to functions specified on the resource type go structs
// if implemented.  See the typeintf package for the implementable functions.
type DefaultStrategy struct {
	Object runtime.Object
	runtime.ObjectTyper
	TableConvertor rest.TableConvertor
}

// GenerateName generates a new name for a resource without one.
func (d DefaultStrategy) GenerateName(base string) string {
	if d.Object == nil {
		return names.SimpleNameGenerator.GenerateName(base)
	}
	if n, ok := d.Object.(names.NameGenerator); ok {
		return n.GenerateName(base)
	}
	return names.SimpleNameGenerator.GenerateName(base)
}

// NamespaceScoped is used to register the resource as namespaced or non-namespaced.
func (d DefaultStrategy) NamespaceScoped() bool {
	if d.Object == nil {
		return true
	}
	if n, ok := d.Object.(rest.Scoper); ok {
		return n.NamespaceScoped()
	}
	return true
}

// ShortNames is used to register short names for easier scripting.
func (d DefaultStrategy) ShortNames() []string {
	if d.Object == nil {
		return nil
	}
	if n, ok := d.Object.(rest.ShortNamesProvider); ok {
		return n.ShortNames()
	}
	return nil
}

// SingularNameProvider
func (d DefaultStrategy) GetSingularName() string {
	if d.Object == nil {
		return ""
	}
	if n, ok := d.Object.(rest.SingularNameProvider); ok {
		return n.GetSingularName()
	}
	return ""
}

// PrepareForCreate calls the PrepareForCreate function on obj if supported, otherwise does nothing.
func (DefaultStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	if v, ok := obj.(resourcestrategy.PrepareForCreater); ok {
		v.PrepareForCreate(ctx)
	}
}

// PrepareForUpdate calls the PrepareForUpdate function on obj if supported, otherwise does nothing.
func (DefaultStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	if v, ok := obj.(resource.ObjectWithStatusSubResource); ok {
		// don't modify the status
		old.(resource.ObjectWithStatusSubResource).GetStatus().CopyTo(v)
	}
	if v, ok := obj.(resourcestrategy.PrepareForUpdater); ok {
		v.PrepareForUpdate(ctx, old)
	}
}

// Validate calls the Validate function on obj if supported, otherwise does nothing.
func (DefaultStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	if v, ok := obj.(resourcestrategy.Validater); ok {
		return v.Validate(ctx)
	}
	return field.ErrorList{}
}

// AllowCreateOnUpdate is used by the Store
func (d DefaultStrategy) AllowCreateOnUpdate() bool {
	if d.Object == nil {
		return false
	}
	if n, ok := d.Object.(resourcestrategy.AllowCreateOnUpdater); ok {
		return n.AllowCreateOnUpdate()
	}
	return false
}

// AllowUnconditionalUpdate is used by the Store
func (d DefaultStrategy) AllowUnconditionalUpdate() bool {
	if d.Object == nil {
		return false
	}
	if n, ok := d.Object.(resourcestrategy.AllowUnconditionalUpdater); ok {
		return n.AllowUnconditionalUpdate()
	}
	return false
}

// Canonicalize calls the Canonicalize function on obj if supported, otherwise does nothing.
func (DefaultStrategy) Canonicalize(obj runtime.Object) {
	if c, ok := obj.(resourcestrategy.Canonicalizer); ok {
		c.Canonicalize()
	}
}

// ValidateUpdate calls the ValidateUpdate function on obj if supported, otherwise does nothing.
func (DefaultStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	if v, ok := obj.(resourcestrategy.ValidateUpdater); ok {
		return v.ValidateUpdate(ctx, old)
	}
	return field.ErrorList{}
}

// Match is the filter used by the generic etcd backend to watch events
// from etcd to clients of the apiserver only interested in specific labels/fields.
func (DefaultStrategy) Match(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}

// ConvertToTable is used for printing the resource from kubectl get
func (d DefaultStrategy) ConvertToTable(
	ctx context.Context, obj runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	if c, ok := obj.(resourcestrategy.TableConverter); ok {
		return c.ConvertToTable(ctx, tableOptions)
	}
	return d.TableConvertor.ConvertToTable(ctx, obj, tableOptions)
}

func (d DefaultStrategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string {
	return nil
}

func (d DefaultStrategy) WarningsOnUpdate(ctx context.Context, old, new runtime.Object) []string {
	return nil
}
