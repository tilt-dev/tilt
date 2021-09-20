/*
Copyright 2020 The Tilt Dev Authors

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

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource/resourcerest"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource/resourcestrategy"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ConfigMap stores unstructured data that other controllers can read and write.
//
// Useful for sharing data from one system and subscribing to it from another.
//
// +k8s:openapi-gen=true
type ConfigMap struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	// Data contains the configuration data.
	// Each key must consist of alphanumeric characters, '-', '_' or '.'.
	// +optional
	Data map[string]string `json:"data,omitempty" protobuf:"bytes,2,rep,name=data"`
}

// ConfigMapList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ConfigMapList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []ConfigMap `json:"items" protobuf:"bytes,2,rep,name=items"`
}

var _ resource.Object = &ConfigMap{}
var _ resourcestrategy.Validater = &ConfigMap{}
var _ resourcerest.ShortNamesProvider = &ConfigMap{}

func (in *ConfigMap) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *ConfigMap) NamespaceScoped() bool {
	return false
}

func (in *ConfigMap) GetSpec() interface{} {
	return nil
}

func (in *ConfigMap) ShortNames() []string {
	return []string{"cm"}
}

func (in *ConfigMap) New() runtime.Object {
	return &ConfigMap{}
}

func (in *ConfigMap) NewList() runtime.Object {
	return &ConfigMapList{}
}

func (in *ConfigMap) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "configmaps",
	}
}

func (in *ConfigMap) IsStorageVersion() bool {
	return true
}

func (in *ConfigMap) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &ConfigMapList{}

func (in *ConfigMapList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}
