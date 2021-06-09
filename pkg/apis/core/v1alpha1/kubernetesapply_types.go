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
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource/resourcestrategy"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KubernetesApply specifies a blob of YAML to apply, and a set of ImageMaps
// that the YAML depends on.
//
// The KubernetesApply controller will resolve the ImageMaps into immutable image
// references. The controller will process the spec YAML, then apply it to the cluster.
// Those processing steps might include:
//
// - Injecting the resolved image references.
// - Adding custom labels so that Tilt can track the progress of the apply.
// - Modifying image pull rules to ensure the image is pulled correctly.
//
// The controller won't apply anything until all ImageMaps resolve to real images.
//
// The controller will watch all the image maps, and redeploy the entire YAML if
// any of the maps resolve to a new image.
//
// The status field will contain both the raw applied object, and derived fields
// to help other controllers figure out how to watch the apply progress.
//
// +k8s:openapi-gen=true
type KubernetesApply struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   KubernetesApplySpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status KubernetesApplyStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// KubernetesApplyList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type KubernetesApplyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []KubernetesApply `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// KubernetesApplySpec defines the desired state of KubernetesApply
type KubernetesApplySpec struct {
}

var _ resource.Object = &KubernetesApply{}
var _ resourcestrategy.Validater = &KubernetesApply{}

func (in *KubernetesApply) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *KubernetesApply) NamespaceScoped() bool {
	return false
}

func (in *KubernetesApply) New() runtime.Object {
	return &KubernetesApply{}
}

func (in *KubernetesApply) NewList() runtime.Object {
	return &KubernetesApplyList{}
}

func (in *KubernetesApply) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "kubernetesapplys",
	}
}

func (in *KubernetesApply) IsStorageVersion() bool {
	return true
}

func (in *KubernetesApply) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &KubernetesApplyList{}

func (in *KubernetesApplyList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// KubernetesApplyStatus defines the observed state of KubernetesApply
type KubernetesApplyStatus struct {
}

// KubernetesApply implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &KubernetesApply{}

func (in *KubernetesApply) GetStatus() resource.StatusSubResource {
	return in.Status
}

// KubernetesApplyStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &KubernetesApplyStatus{}

func (in KubernetesApplyStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*KubernetesApply).Status = in
}
