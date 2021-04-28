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
	"fmt"

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

// PortForward
// +k8s:openapi-gen=true
type PortForward struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PortForwardSpec   `json:"spec,omitempty"`
	Status PortForwardStatus `json:"status,omitempty"`
}

// PortForwardList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type PortForwardList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	Items []PortForward `json:"items"`
}

// PortForwardSpec defines the desired state of PortForward
type PortForwardSpec struct {
	// The name of the pod to port forward to/from. Required.
	PodName string `json:"pod_name"`

	// The namespace of the pod to port forward to/from. Defaults to the kubecontext default namespace.
	//
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// The port to expose on the current machine. Required.
	LocalPort int `json:"local_port"`

	// The port on the Kubernetes pod to connect to. Required.
	ContainerPort int `json:"container_port"`

	// Optional host to bind to on the current machine (localhost by default)
	//
	// +optional
	Host string `json:"host"`
}

var _ resource.Object = &PortForward{}
var _ resourcestrategy.Validater = &PortForward{}

func NewPortForward(localPort, containerPort int, host string, podID string, ns string, mName string) *PortForward {
	// generate a consistent name for a port forward based on its component parts
	name := pfName(localPort, containerPort, host, podID, ns, mName)

	return &PortForward{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Annotations: map[string]string{
				// Name of the manifest that this Port Forward corresponds to
				// (we need this to route the logs correctly)
				AnnotationManifest: mName,
			},
		},
		Spec: PortForwardSpec{
			PodName:       podID,
			Namespace:     ns,
			ContainerPort: containerPort,
			LocalPort:     localPort,
			Host:          host,
		},
	}
}

// pfName generates a consistent name for a port forward based on its component parts
// NOTE(maia): the name is super detailed because right now the PortForwardSubscriber can
//   only create OR delete a PF by name, not both (i.e. can't delete the old version of a PF
//   with the given name and then re-create the new version), so a name needs to uniquely
//   identify a possible PF. Maybe as we continue the migration this can chill out a bit.
func pfName(localPort, containerPort int, host string, podID string, ns string, mName string) string {
	if host == "" {
		host = "localhost"
	}
	name := fmt.Sprintf("%s-%s:%d-%d-%s", mName, host, localPort, containerPort, podID)
	if ns != "" {
		name = fmt.Sprintf("%s-%s", name, ns)
	}
	return name
}

func (in *PortForward) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *PortForward) NamespaceScoped() bool {
	return false
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

func (in *PortForward) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	// TODO(maia): verify that Pod is populated, ContainerPort and LocalPort are
	//   non-zero, (maybe) that host (if populated) is URL-parse-able.
	return nil
}

var _ resource.ObjectList = &PortForwardList{}

func (in *PortForwardList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// PortForwardStatus defines the observed state of PortForward
type PortForwardStatus struct {
	// Time at which we started trying to run the Port Forward (potentially distinct
	// from the time the Port Forward successfully connected)
	StartedAt metav1.MicroTime `json:"startedAt,omitempty"`

	// TODO(maia): track status of the PortForward: is it active, is it failing/in
	//   backoff, what was the last error? Exact fields/status TBD.
	//   (Need to figure out the right place to write this data without lots of
	//   churn/without re-writing every time we fail to connect and back off.)
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
