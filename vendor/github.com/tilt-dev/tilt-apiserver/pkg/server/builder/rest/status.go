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

	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource/util"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var _ Strategy = StatusSubResourceStrategy{}

// StatusSubResourceStrategy defines a default Strategy for the status subresource.
type StatusSubResourceStrategy struct {
	Strategy
}

// PrepareForUpdate calls the PrepareForUpdate function on obj if supported, otherwise does nothing.
func (s StatusSubResourceStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
	// should panic/fail-fast upon casting failure
	statusObj := obj.(resource.ObjectWithStatusSubResource)
	statusOld := old.(resource.ObjectWithStatusSubResource)
	// only modifies status
	statusObj.GetStatus().CopyTo(statusOld)
	if err := util.DeepCopy(statusOld, statusObj); err != nil {
		utilruntime.HandleError(err)
	}
}
