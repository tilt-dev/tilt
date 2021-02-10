/*
Copyright 2017 The Kubernetes Authors.

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

package filepath

import (
	"context"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/storage/names"
)

// NewStrategy creates and returns a genericStrategy instance
func NewStrategy(typer runtime.ObjectTyper, obj resource.Object) genericStrategy {
	return genericStrategy{typer, names.SimpleNameGenerator, obj}
}

type genericStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
	obj resource.Object
}

func (s genericStrategy) NamespaceScoped() bool {
	return s.obj.NamespaceScoped()
}

func (genericStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
}

func (genericStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
}

func (genericStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	return field.ErrorList{}
}

func (genericStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (genericStrategy) AllowUnconditionalUpdate() bool {
	return false
}

func (genericStrategy) Canonicalize(obj runtime.Object) {
}

func (genericStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return field.ErrorList{}
}
