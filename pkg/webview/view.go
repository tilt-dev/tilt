package webview

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type TiltBuild struct {
	Version   string `json:"version,omitempty"`
	CommitSHA string `json:"commitSHA,omitempty"`
	Date      string `json:"date,omitempty"`
	Dev       bool   `json:"dev,omitempty"`
}

type VersionSettings struct {
	CheckUpdates bool `json:"checkUpdates,omitempty"`
}

// Our websocket service has two kinds of View messages:
//
//  1. On initialization, we send down the complete view state
//     (TiltStartTime, UISession, UIResources, and LogList)
//
//  2. On every change, we send down the resources that have
//     changed since the last send().
//     (new logs and any updated UISession/UIResource objects).
//
// All other fields are obsolete, but are needed for deserializing
// old snapshots.
type View struct {
	Log                  string           `json:"log,omitempty"`
	FeatureFlags         map[string]bool  `json:"featureFlags,omitempty"`
	NeedsAnalyticsNudge  bool             `json:"needsAnalyticsNudge,omitempty"`
	RunningTiltBuild     *TiltBuild       `json:"runningTiltBuild,omitempty"`
	SuggestedTiltVersion string           `json:"suggestedTiltVersion,omitempty"`
	VersionSettings      *VersionSettings `json:"versionSettings,omitempty"`
	TiltCloudUsername    string           `json:"tiltCloudUsername,omitempty"`
	TiltCloudTeamName    string           `json:"tiltCloudTeamName,omitempty"`
	TiltCloudSchemeHost  string           `json:"tiltCloudSchemeHost,omitempty"`
	TiltCloudTeamID      string           `json:"tiltCloudTeamID,omitempty"`
	FatalError           string           `json:"fatalError,omitempty"`
	LogList              *LogList         `json:"logList,omitempty"`
	// Allows us to synchronize on a running Tilt instance,
	// so we can tell when Tilt restarted.
	TiltStartTime metav1.MicroTime `json:"tiltStartTime,omitempty"`

	// An identifier for the tiltfile that is running, so that the web ui
	// can store data per tiltfile.
	TiltfileKey string `json:"tiltfileKey,omitempty"`

	// New API-server based data models.
	UiSession   *v1alpha1.UISession   `json:"uiSession,omitempty"`
	UiResources []v1alpha1.UIResource `json:"uiResources,omitempty"`
	UiButtons   []v1alpha1.UIButton   `json:"uiButtons,omitempty"`
	Clusters    []v1alpha1.Cluster    `json:"clusters,omitempty"`

	// Indicates that this view is a complete representation of the app.
	// If false, this view just contains deltas from a previous view.
	IsComplete bool `json:"isComplete,omitempty"`
}

type SnapshotHighlight struct {
	BeginningLogID string `json:"beginningLogID,omitempty"`
	EndingLogID    string `json:"endingLogID,omitempty"`
	Text           string `json:"text,omitempty"`
}

type Snapshot struct {
	View              *View              `json:"view,omitempty"`
	IsSidebarClosed   bool               `json:"isSidebarClosed,omitempty"`
	Path              string             `json:"path,omitempty"`
	SnapshotHighlight *SnapshotHighlight `json:"snapshotHighlight,omitempty"`
	SnapshotLink      string             `json:"snapshotLink,omitempty"`
	CreatedAt         metav1.MicroTime   `json:"createdAt,omitempty"`
}

type UploadSnapshotResponse struct {
	Url string `json:"url,omitempty"`
}

// NOTE(nick): This is obsolete.
//
// Our websocket service has two kinds of messages:
//  1. On initialization, we send down the complete view state
//  2. On every change, we send down the resources that have
//     changed since the last send().
type AckWebsocketRequest struct {
	// The ToCheckpoint on the received LogList.
	ToCheckpoint int32 `json:"toCheckpoint,omitempty"`

	// Allows us to synchronize on a running Tilt instance,
	// so we can tell when we're talking to the same Tilt.
	TiltStartTime metav1.MicroTime `json:"tiltStartTime,omitempty"`
}

type AckWebsocketResponse struct{}
