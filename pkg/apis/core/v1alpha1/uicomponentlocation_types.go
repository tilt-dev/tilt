/*
Copyright 2015 The Kubernetes Authors.
Copyright 2021 The Tilt Dev Authors.

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

package v1alpha1

// Specifies where to put a UI Component
type UIComponentLocation struct {
	// which resource to place the component on
	Resource *UIComponentLocationResource `json:"resource"`
	// If there are multiple components in the same location, they will be
	// located in order from lowest to highest. Ties are broken arbitrarily.
	Order int `json:"order"`
}

type UIComponentLocationResource struct {
	ResourceName string `json:"resourceName"`
}
