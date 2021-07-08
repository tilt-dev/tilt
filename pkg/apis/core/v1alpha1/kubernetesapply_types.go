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
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource/resourcerest"
	"github.com/tilt-dev/tilt-apiserver/pkg/server/builder/resource/resourcestrategy"
)

const KubernetesApplyTimeoutDefault = 30 * time.Second

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
	// The YAML to apply to the cluster. Required.
	YAML string `json:"yaml" protobuf:"bytes,1,opt,name=yaml"`

	// Names of image maps that this applier depends on.
	//
	// The controller will watch all the image maps, and redeploy the entire YAML
	// if any of the maps resolve to a new image.
	//
	// +optional
	ImageMaps []string `json:"imageMaps,omitempty" protobuf:"bytes,2,rep,name=imageMaps"`

	// Descriptors of how to find images in the YAML.
	//
	// Needed when injecting images into CRDs.
	//
	// +optional
	ImageLocators []KubernetesImageLocator `json:"imageLocators,omitempty" protobuf:"bytes,3,rep,name=imageLocators"`

	// The timeout on the apply operation.
	//
	// We've had problems with both:
	// 1) CRD apiservers that take an arbitrarily long time to apply, and
	// 2) Infinite loops in the apimachinery
	// So we offer the ability to set a timeout on Kubernetes apply operations.
	//
	// The default timeout is 30s.
	//
	// +optional
	Timeout metav1.Duration `json:"timeout,omitempty" protobuf:"bytes,4,opt,name=timeout"`

	// KubernetesDiscoveryTemplateSpec describes how we discover pods
	// for resources created by this Apply.
	//
	// If not specified, the KubernetesDiscovery controller will listen to all pods,
	// and follow owner references to find the pods owned by these resources.
	//
	// +optional
	KubernetesDiscoveryTemplateSpec *KubernetesDiscoveryTemplateSpec `json:"kubernetesDiscoveryTemplateSpec,omitempty" protobuf:"bytes,5,opt,name=kubernetesDiscoveryTemplateSpec"`

	// PortForwardTemplateSpec describes the data model for port forwards
	// that KubernetesApply should set up.
	//
	// Underneath the hood, we'll create a KubernetesDiscovery object that finds
	// the pods and sets up the port-forwarding. Only one PortForward will be
	// active at a time.
	//
	// +optional
	PortForwardTemplateSpec *PortForwardTemplateSpec `json:"portForwardTemplateSpec,omitempty" protobuf:"bytes,6,opt,name=portForwardTemplateSpec"`

	// PodLogStreamTemplateSpec describes the data model for PodLogStreams
	// that KubernetesApply should set up.
	//
	// Underneath the hood, we'll create a KubernetesDiscovery object that finds
	// the pods and sets up the pod log streams.
	//
	// If no template is specified, the controller will stream all
	// pod logs available from the apiserver.
	//
	// +optional
	PodLogStreamTemplateSpec *PodLogStreamTemplateSpec `json:"podLogStreamTemplateSpec,omitempty" protobuf:"bytes,7,opt,name=podLogStreamTemplateSpec"`
}

var _ resource.Object = &KubernetesApply{}
var _ resourcestrategy.Validater = &KubernetesApply{}
var _ resourcerest.ShortNamesProvider = &KubernetesApply{}

func (in *KubernetesApply) GetSpec() interface{} {
	return in.Spec
}

func (in *KubernetesApply) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *KubernetesApply) NamespaceScoped() bool {
	return false
}

func (in *KubernetesApply) ShortNames() []string {
	return []string{"ka", "kapp"}
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
	var fieldErrors field.ErrorList
	if in.Spec.YAML == "" {
		fieldErrors = append(fieldErrors, field.Required(field.NewPath("yaml"), "cannot be empty"))
	}

	// TODO(nick): Validate the image locators as well.

	return fieldErrors
}

var _ resource.ObjectList = &KubernetesApplyList{}

func (in *KubernetesApplyList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// KubernetesApplyStatus defines the observed state of KubernetesApply
type KubernetesApplyStatus struct {
	// The result of applying the YAML to the cluster. This should contain
	// UIDs for the applied resources.
	//
	// +optional
	ResultYAML string `json:"resultYAML,omitempty" protobuf:"bytes,1,opt,name=resultYAML"`

	// An error applying the YAML.
	//
	// If there was an error, than ResultYAML should be empty (and vice versa).
	//
	// +optional
	Error string `json:"error,omitempty" protobuf:"bytes,2,opt,name=error"`

	// The last time the controller tried to apply YAML.
	//
	// +optional
	LastApplyTime metav1.MicroTime `json:"lastApplyTime,omitempty" protobuf:"bytes,3,opt,name=lastApplyTime"`

	// A base64-encoded hash of all the inputs to the apply.
	//
	// We added this so that more procedural code can determine whether
	// their updates have been applied yet or not by the reconciler. But any code
	// using it this way should note that the reconciler may "skip" an update
	// (e.g., if two images get updated in quick succession before the reconciler
	// injects them into the YAML), so a particular ApplieInputHash might never appear.
	//
	// +optional
	AppliedInputHash string `json:"appliedInputHash,omitempty" protobuf:"bytes,4,opt,name=appliedInputHash"`

	// TODO(nick): We should also add some sort of status field to this
	// status (like waiting, active, done).
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

// Finds image references in Kubernetes YAML.
type KubernetesImageLocator struct {
	// Selects which objects to look in.
	ObjectSelector ObjectSelector `json:"objectSelector" protobuf:"bytes,1,opt,name=objectSelector"`

	// A JSON path to the image reference field.
	//
	// If Object is empty, the field should be a string.
	//
	// If Object is non-empty, the field should be an object with subfields.
	Path string `json:"path" protobuf:"bytes,2,opt,name=path"`

	// A descriptor of the path and structure of an object that describes an image
	// reference. This is a common way to describe images in CRDs, breaking
	// them down into an object rather than an image reference string.
	//
	// +optional
	Object *KubernetesImageObjectDescriptor `json:"object,omitempty" protobuf:"bytes,3,opt,name=object"`
}

type KubernetesImageObjectDescriptor struct {
	// The name of the field that contains the image repository.
	RepoField string `json:"repoField" protobuf:"bytes,1,opt,name=repoField"`

	// The name of the field that contains the image tag.
	TagField string `json:"tagField" protobuf:"bytes,2,opt,name=tagField"`
}

type KubernetesDiscoveryTemplateSpec struct {
	// ExtraSelectors are label selectors that will force discovery of a Pod even
	// if it does not match the AncestorUID.
	//
	// This should only be necessary in the event that a CRD creates Pods but does
	// not set an owner reference to itself.
	ExtraSelectors []metav1.LabelSelector `json:"extraSelectors,omitempty" protobuf:"bytes,1,rep,name=extraSelectors"`
}
