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

// PortForward
// +k8s:openapi-gen=true
type PortForward struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   PortForwardSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status PortForwardStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// PortForwardList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PortForwardList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []PortForward `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// PortForwardSpec defines the desired state of PortForward
type PortForwardSpec struct {
	// The name of the pod to port forward to/from. Required.
	PodName string `json:"podName" protobuf:"bytes,1,opt,name=podName"`

	// The namespace of the pod to port forward to/from. Defaults to the kubecontext default namespace.
	//
	// +optional
	Namespace string `json:"namespace,omitempty" protobuf:"bytes,2,opt,name=namespace"`

	// One or more port forwards to execute on the given pod. Required.
	Forwards []Forward `json:"forwards" protobuf:"bytes,3,rep,name=forwards"`

	// Cluster to forward ports from to the local machine.
	//
	// If not specified, the default Kubernetes cluster will be used.
	//
	// +optional
	Cluster string `json:"cluster" protobuf:"bytes,4,opt,name=cluster"`
}

// Forward defines a port forward to execute on a given pod.
type Forward struct {
	// The port to expose on the current machine.
	//
	// If not specified (or 0), a random free port will be chosen and can
	// be discovered via the status once established.
	//
	// +optional
	LocalPort int32 `json:"localPort,omitempty" protobuf:"varint,4,opt,name=localPort"`

	// The port on the Kubernetes pod to connect to. Required.
	ContainerPort int32 `json:"containerPort" protobuf:"varint,3,opt,name=containerPort"`

	// Optional host to bind to on the current machine.
	//
	// If not explicitly specified, uses the bind host of the tilt web UI (usually localhost).
	//
	// +optional
	Host string `json:"host" protobuf:"bytes,5,opt,name=host"`

	// Name to identify this port forward.
	//
	// +optional
	Name string `json:"name,omitempty" protobuf:"bytes,6,opt,name=name"`

	// Path to include as part of generated links for port forward.
	//
	// +optional
	Path string `json:"path,omitempty" protobuf:"bytes,7,opt,name=path"`
}

var _ resource.Object = &PortForward{}
var _ resourcerest.SingularNameProvider = &PortForward{}
var _ resourcestrategy.Validater = &PortForward{}
var _ resourcerest.ShortNamesProvider = &PortForward{}

func (in *PortForward) GetSingularName() string {
	return "portforward"
}

func (in *PortForward) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *PortForward) NamespaceScoped() bool {
	return false
}

func (in *PortForward) ShortNames() []string {
	return []string{"pf"}
}

func (in *PortForward) New() runtime.Object {
	return &PortForward{}
}

func (in *PortForward) NewList() runtime.Object {
	return &PortForwardList{}
}

func (in *PortForward) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "portforwards",
	}
}

func (in *PortForward) IsStorageVersion() bool {
	return true
}

func (in *PortForward) Validate(_ context.Context) field.ErrorList {
	var fieldErrors field.ErrorList
	if in.Spec.PodName == "" {
		fieldErrors = append(fieldErrors, field.Required(field.NewPath("spec.podName"), "PodName cannot be empty"))
	}
	forwardsPath := field.NewPath("spec.forwards")
	if len(in.Spec.Forwards) == 0 {
		fieldErrors = append(fieldErrors, field.Required(forwardsPath, "At least one Forward is required"))
	}

	localPorts := make(map[int32]bool)
	for i, f := range in.Spec.Forwards {
		p := forwardsPath.Index(i)
		localPortPath := p.Child("localPort")
		if f.LocalPort != 0 {
			// multiple forwards can have 0 as LocalPort since they will each get a unique, randomized port
			// there is no restriction for duplicate ContainerPorts (i.e. it's acceptable to forward the same
			// port multiple times as long as the LocalPort is different in each forward)
			if localPorts[f.LocalPort] {
				fieldErrors = append(fieldErrors, field.Duplicate(localPortPath,
					"Cannot bind more than one forward to same LocalPort"))
			}
			localPorts[f.LocalPort] = true
		}
		if f.LocalPort < 0 || f.LocalPort > 65535 {
			fieldErrors = append(fieldErrors, field.Invalid(localPortPath, f.LocalPort,
				"LocalPort must be in the range [0, 65535]"))
		}

		if f.ContainerPort <= 0 || f.ContainerPort > 65535 {
			fieldErrors = append(fieldErrors, field.Invalid(p.Child("containerPort"), f.ContainerPort,
				"ContainerPort must be in the range (0, 65535]"))
		}
	}

	return fieldErrors
}

var _ resourcestrategy.Defaulter = &PortForward{}

func (in *PortForward) Default() {
	if in.Spec.Cluster == "" {
		in.Spec.Cluster = ClusterNameDefault
	}
}

var _ resource.ObjectList = &PortForwardList{}

func (in *PortForwardList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// PortForwardStatus defines the observed state of PortForward
type PortForwardStatus struct {
	ForwardStatuses []ForwardStatus `json:"forwardStatuses,omitempty" protobuf:"bytes,2,opt,name=forwardStatuses"`
}

type ForwardStatus struct {
	// LocalPort is the port bound to on the system running Tilt.
	LocalPort int32 `json:"localPort" protobuf:"varint,1,opt,name=localPort"`

	// ContainerPort is the port in the container being forwarded.
	ContainerPort int32 `json:"containerPort" protobuf:"varint,2,opt,name=containerPort"`

	// Addresses that the forwarder is bound to.
	//
	// For example, a `localhost` host will bind to 127.0.0.1 and [::1].
	Addresses []string `json:"addresses" protobuf:"bytes,3,rep,name=addresses"`

	// StartedAt is the time at which the forward was initiated.
	//
	// If the forwarder is not running yet, this will be zero/empty.
	StartedAt metav1.MicroTime `json:"startedAt,omitempty" protobuf:"bytes,4,opt,name=startedAt"`

	// Error is a human-readable description if a problem was encountered
	// while initializing the forward.
	Error string `json:"error,omitempty" protobuf:"bytes,5,opt,name=error"`
}

// PortForward implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &PortForward{}

func (in *PortForward) GetStatus() resource.StatusSubResource {
	return in.Status
}

// PortForwardStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &PortForwardStatus{}

func (in PortForwardStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*PortForward).Status = in
}
