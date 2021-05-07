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

// UIResource represents per-resource status data for rendering the web UI.
//
// Treat this as a legacy data structure that's more intended to make transition
// easier rather than a robust long-term API.
//
// +k8s:openapi-gen=true
type UIResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   UIResourceSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status UIResourceStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// UIResourceList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type UIResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []UIResource `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// UIResourceSpec is an empty struct.
// UIResource is a kludge for making Tilt's internal status readable, not
// for specifying behavior.
type UIResourceSpec struct {
}

var _ resource.Object = &UIResource{}
var _ resourcestrategy.Validater = &UIResource{}

func (in *UIResource) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *UIResource) NamespaceScoped() bool {
	return false
}

func (in *UIResource) New() runtime.Object {
	return &UIResource{}
}

func (in *UIResource) NewList() runtime.Object {
	return &UIResourceList{}
}

func (in *UIResource) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "uiresources",
	}
}

func (in *UIResource) IsStorageVersion() bool {
	return true
}

func (in *UIResource) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &UIResourceList{}

func (in *UIResourceList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// UIResourceStatus defines the observed state of UIResource
type UIResourceStatus struct {
	// The last time this resource was deployed.
	LastDeployTime metav1.MicroTime `json:"lastDeployTime,omitempty" protobuf:"bytes,1,opt,name=lastDeployTime"`

	// Bit mask representing whether this resource is run when:
	// 1) When a file changes
	// 2) When the resource initializes
	TriggerMode int32 `json:"triggerMode,omitempty" protobuf:"varint,2,opt,name=triggerMode"`

	// Past completed builds.
	BuildHistory []UIBuildTerminated `json:"buildHistory,omitempty" protobuf:"bytes,3,rep,name=buildHistory"`

	// The currently running build, if any.
	CurrentBuild *UIBuildRunning `json:"currentBuild,omitempty" protobuf:"bytes,4,opt,name=currentBuild"`

	// When the build was put in the pending queue.
	PendingBuildSince metav1.MicroTime `json:"pendingBuildSince,omitempty" protobuf:"bytes,5,opt,name=pendingBuildSince"`

	// True if the build was put in the pending queue due to file changes.
	HasPendingChanges bool `json:"hasPendingChanges,omitempty" protobuf:"varint,6,opt,name=hasPendingChanges"`

	// Links attached to this resource.
	EndpointLinks []UIResourceLink `json:"endpointLinks,omitempty" protobuf:"bytes,7,rep,name=endpointLinks"`

	// Extra data about Kubernetes resources.
	K8sResourceInfo *UIResourceKubernetes `json:"k8sResourceInfo,omitempty" protobuf:"bytes,8,opt,name=k8sResourceInfo"`

	// Extra data about Local resources
	LocalResourceInfo *UIResourceLocal `json:"localResourceInfo,omitempty" protobuf:"bytes,9,opt,name=localResourceInfo"`

	// The RuntimeStatus is a simple, high-level summary of the runtime state of a server.
	//
	// Not all resources run servers.
	RuntimeStatus RuntimeStatus `json:"runtimeStatus,omitempty" protobuf:"bytes,10,opt,name=runtimeStatus,casttype=RuntimeStatus"`

	// The UpdateStatus is a simple, high-level summary of any update tasks to bring
	// the resource up-to-date.
	//
	// If the resource runs a server, this may include both build tasks and live-update
	// syncing.
	UpdateStatus UpdateStatus `json:"updateStatus,omitempty" protobuf:"bytes,14,opt,name=updateStatus,casttype=UpdateStatus"`

	// Information about all the target specs that this resource summarizes.
	Specs []UIResourceTargetSpec `json:"specs,omitempty" protobuf:"bytes,12,rep,name=specs"`

	// Queued is a simple indicator of whether the resource is queued for an update.
	Queued bool `json:"queued,omitempty" protobuf:"varint,13,opt,name=queued"`
}

// UIResource implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &UIResource{}

func (in *UIResource) GetStatus() resource.StatusSubResource {
	return in.Status
}

// UIResourceStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &UIResourceStatus{}

func (in UIResourceStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*UIResource).Status = in
}

// UIResourceLink represents a link assocatiated with a UIResource.
type UIResourceLink struct {
	// A URL to link to.
	URL string `json:"url,omitempty" protobuf:"bytes,1,opt,name=url"`

	// The display label on a URL.
	Name string `json:"name,omitempty" protobuf:"bytes,2,opt,name=name"`
}

// UIResourceTargetType identifies the different categories of
// task in a resource.
type UIResourceTargetType string

const (
	// The target type is unspecified.
	UIResourceTargetTypeUnspecified UIResourceTargetType = "unspecified"

	// The target is a container image build.
	UIResourceTargetTypeImage UIResourceTargetType = "image"

	// The target is a Kubernetes resource deployment.
	UIResourceTargetTypeKubernetes UIResourceTargetType = "k8s"

	// The target is a Docker Compose service deployment.
	UIResourceTargetTypeDockerCompose UIResourceTargetType = "docker-compose"

	// The target is a local command or server.
	UIResourceTargetTypeLocal UIResourceTargetType = "local"
)

// UIResourceTargetSpec represents the spec of a build or deploy that a resource summarizes.
type UIResourceTargetSpec struct {
	// The ID of the target.
	ID string `json:"id,omitempty" protobuf:"bytes,1,opt,name=id"`

	// The type of the target.
	Type UIResourceTargetType `json:"type,omitempty" protobuf:"bytes,2,opt,name=type,casttype=UIResourceTargetType"`

	// Whether the target has a live update assocated with it.
	HasLiveUpdate bool `json:"hasLiveUpdate,omitempty" protobuf:"varint,3,opt,name=hasLiveUpdate"`
}

// UIBuildRunning respresents an in-progress build/update in the user interface.
type UIBuildRunning struct {
	// The time when the build started.
	StartTime metav1.MicroTime `json:"startTime,omitempty" protobuf:"bytes,1,opt,name=startTime"`

	// The log span where the build logs are stored in the logstore.
	SpanID string `json:"spanID,omitempty" protobuf:"bytes,2,opt,name=spanID"`
}

// UIBuildRunning respresents a finished build/update in the user interface.
type UIBuildTerminated struct {
	// A non-empty string if the build failed with an error.
	Error string `json:"error,omitempty" protobuf:"bytes,1,opt,name=error"`

	// A list of warnings encountered while running the build.
	// These warnings will also be printed to the build's log.
	Warnings []string `json:"warnings,omitempty" protobuf:"bytes,2,rep,name=warnings"`

	// The time when the build started.
	StartTime metav1.MicroTime `json:"startTime,omitempty" protobuf:"bytes,3,opt,name=startTime"`

	// The time when the build finished.
	FinishTime metav1.MicroTime `json:"finishTime,omitempty" protobuf:"bytes,4,opt,name=finishTime"`

	// The log span where the build logs are stored in the logstore.
	SpanID string `json:"spanID,omitempty" protobuf:"bytes,5,opt,name=spanID"`

	// A crash rebuild happens when Tilt live-updated a container, then
	// the pod crashed, wiping out the live-updates. Tilt does a full
	// build+deploy to reset the pod state to what's on disk.
	IsCrashRebuild bool `json:"isCrashRebuild,omitempty" protobuf:"varint,6,opt,name=isCrashRebuild"`
}

// UIResourceKubernetes contains status information specific to Kubernetes.
type UIResourceKubernetes struct {
	// The name of the active pod.
	//
	// The active pod tends to be what Tilt defaults to for port-forwards,
	// live-updates, etc.
	PodName string `json:"podName,omitempty" protobuf:"bytes,1,opt,name=podName"`

	// The creation time of the active pod.
	PodCreationTime metav1.Time `json:"podCreationTime,omitempty" protobuf:"bytes,2,opt,name=podCreationTime"`

	// The last update time of the active pod
	PodUpdateStartTime metav1.Time `json:"podUpdateStartTime,omitempty" protobuf:"bytes,3,opt,name=podUpdateStartTime"`

	// The status of the active pod.
	PodStatus string `json:"podStatus,omitempty" protobuf:"bytes,4,opt,name=podStatus"`

	// Extra error messaging around the current status of the active pod.
	PodStatusMessage string `json:"podStatusMessage,omitempty" protobuf:"bytes,5,opt,name=podStatusMessage"`

	// Whether all the containers in the pod are currently healthy
	// and have passed readiness checks.
	AllContainersReady bool `json:"allContainersReady,omitempty" protobuf:"varint,6,opt,name=allContainersReady"`

	// The number of pod restarts.
	PodRestarts int32 `json:"podRestarts,omitempty" protobuf:"varint,7,opt,name=podRestarts"`

	// The span where this pod stores its logs in the Tilt logstore.
	SpanID string `json:"spanID,omitempty" protobuf:"bytes,8,opt,name=spanID"`

	// The list of all resources deployed in the Kubernetes deploy
	// for this resource.
	DisplayNames []string `json:"displayNames,omitempty" protobuf:"bytes,9,rep,name=displayNames"`
}

// UIResourceLocal contains status information specific to local commands.
type UIResourceLocal struct {
	// The PID of the actively running local command.
	PID int64 `json:"pid,omitempty" protobuf:"varint,1,opt,name=pid"`

	// Whether this represents a test job.
	IsTest bool `json:"isTest,omitempty" protobuf:"varint,2,opt,name=isTest"`
}
