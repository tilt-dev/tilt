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

// UISession represents global status data for rendering the web UI.
//
// Treat this as a legacy data structure that's more intended to make transition
// easier rather than a robust long-term API.
//
// Per-resource status data should be stored in UIResource.
//
// +k8s:openapi-gen=true
type UISession struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Spec   UISessionSpec   `json:"spec,omitempty" protobuf:"bytes,2,opt,name=spec"`
	Status UISessionStatus `json:"status,omitempty" protobuf:"bytes,3,opt,name=status"`
}

// UISessionList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type UISessionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty" protobuf:"bytes,1,opt,name=metadata"`

	Items []UISession `json:"items" protobuf:"bytes,2,rep,name=items"`
}

// UISessionSpec is an empty struct.
// UISession is a kludge for making Tilt's internal status readable, not
// for specifying behavior.
type UISessionSpec struct {
}

var _ resource.Object = &UISession{}
var _ resourcestrategy.Validater = &UISession{}

func (in *UISession) GetObjectMeta() *metav1.ObjectMeta {
	return &in.ObjectMeta
}

func (in *UISession) NamespaceScoped() bool {
	return false
}

func (in *UISession) New() runtime.Object {
	return &UISession{}
}

func (in *UISession) NewList() runtime.Object {
	return &UISessionList{}
}

func (in *UISession) GetGroupVersionResource() schema.GroupVersionResource {
	return schema.GroupVersionResource{
		Group:    "tilt.dev",
		Version:  "v1alpha1",
		Resource: "uisessions",
	}
}

func (in *UISession) IsStorageVersion() bool {
	return true
}

func (in *UISession) Validate(ctx context.Context) field.ErrorList {
	// TODO(user): Modify it, adding your API validation here.
	return nil
}

var _ resource.ObjectList = &UISessionList{}

func (in *UISessionList) GetListMeta() *metav1.ListMeta {
	return &in.ListMeta
}

// UISessionStatus defines the observed state of UISession
type UISessionStatus struct {
	// FeatureFlags reports a list of experimental features that have been
	// enabled.
	FeatureFlags []UIFeatureFlag `json:"featureFlags" protobuf:"bytes,1,rep,name=featureFlags"`

	// NeedsAnalyticsNudge reports whether the UI hasn't opted in or out
	// of analytics, and the UI should nudge them to do so.
	NeedsAnalyticsNudge bool `json:"needsAnalyticsNudge" protobuf:"varint,2,opt,name=needsAnalyticsNudge"`

	// RunningTiltBuild reports the currently running version of tilt
	// that this UI is talking to.
	RunningTiltBuild TiltBuild `json:"runningTiltBuild" protobuf:"bytes,3,opt,name=runningTiltBuild"`

	// SuggestedTiltVersion tells the UI the recommended version for this
	// user. If the version is different than what's running, the UI
	// may display a prompt to upgrade.
	SuggestedTiltVersion string `json:"suggestedTiltVersion" protobuf:"bytes,4,opt,name=suggestedTiltVersion"`

	// VersionSettings indicates whether version updates have been enabled/disabled
	// from the Tiltfile.
	VersionSettings VersionSettings `json:"versionSettings" protobuf:"bytes,12,opt,name=versionSettings"`

	// TiltCloudUsername reports the username if the user is signed into
	// TiltCloud.
	TiltCloudUsername string `json:"tiltCloudUsername" protobuf:"bytes,5,opt,name=tiltCloudUsername"`

	// TiltCloudUsername reports the human-readable team name if the user is
	// signed into TiltCloud and the Tiltfile declares a team.
	TiltCloudTeamName string `json:"tiltCloudTeamName" protobuf:"bytes,6,opt,name=tiltCloudTeamName"`

	// TiltCloudSchemeHost reports the base URL of the Tilt Cloud instance
	// associated with this Tilt process. Usually https://cloud.tilt.dev
	TiltCloudSchemeHost string `json:"tiltCloudSchemeHost" protobuf:"bytes,7,opt,name=tiltCloudSchemeHost"`

	// TiltCloudTeamID reports the unique team id if the user is signed into
	// TiltCloud and the Tiltfile declares a team.
	TiltCloudTeamID string `json:"tiltCloudTeamID" protobuf:"bytes,8,opt,name=tiltCloudTeamID"`

	// A FatalError is an error that forces Tilt to stop its control loop.
	// The API server will stay up and continue to serve the UI, but
	// no further builds will happen.
	FatalError string `json:"fatalError" protobuf:"bytes,9,opt,name=fatalError"`

	// The time that this instance of tilt started.
	// Clients can use this to determine if the API server has restarted
	// and all the objects need to be refreshed.
	TiltStartTime metav1.Time `json:"tiltStartTime" protobuf:"bytes,10,opt,name=tiltStartTime"`

	// An identifier for the Tiltfile that is running.
	// Clients can use this to store data associated with a particular
	// project in LocalStorage or other persistent storage.
	TiltfileKey string `json:"tiltfileKey" protobuf:"bytes,11,opt,name=tiltfileKey"`
}

// UISession implements ObjectWithStatusSubResource interface.
var _ resource.ObjectWithStatusSubResource = &UISession{}

func (in *UISession) GetStatus() resource.StatusSubResource {
	return in.Status
}

// UISessionStatus{} implements StatusSubResource interface.
var _ resource.StatusSubResource = &UISessionStatus{}

func (in UISessionStatus) CopyTo(parent resource.ObjectWithStatusSubResource) {
	parent.(*UISession).Status = in
}

// Configures Tilt to enable non-default features (e.g., experimental or
// deprecated).
//
// The Tilt features controlled by this are generally in an unfinished state,
// and not yet documented.
//
// As a Tilt user, you don’t need to worry about this unless something
// else directs you to (e.g., an experimental feature doc, or a conversation
// with a Tilt contributor).
type UIFeatureFlag struct {
	// The name of the flag.
	Name string `json:"name" protobuf:"bytes,1,opt,name=name"`

	// The value of the flag.
	Value bool `json:"value" protobuf:"varint,2,opt,name=value"`
}

// Information about the running tilt binary.
type TiltBuild struct {
	// A semantic version string.
	Version string `json:"version" protobuf:"bytes,1,opt,name=version"`

	// The Git digest of the commit this binary was built at.
	CommitSHA string `json:"commitSHA" protobuf:"bytes,2,opt,name=commitSHA"`

	// A human-readable string representing when the binary was built.
	Date string `json:"date" protobuf:"bytes,3,opt,name=date"`

	// Indicates whether this is a development build (true) or an official release (false).
	Dev bool `json:"dev" protobuf:"varint,4,opt,name=dev"`
}

// Information about how the Tilt binary handles updates.
type VersionSettings struct {
	// Whether version updates have been enabled/disabled from the Tiltfile.
	CheckUpdates bool `json:"checkUpdates" protobuf:"varint,1,opt,name=checkUpdates"`
}
