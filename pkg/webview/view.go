package webview

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1alpha1 "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

// TargetType corresponds to implementations of the TargetSpec interface.
type TargetType string

const (
	TargetType_TARGET_TYPE_UNSPECIFIED    TargetType = "unspecified"
	TargetType_TARGET_TYPE_IMAGE          TargetType = "image"
	TargetType_TARGET_TYPE_K8S            TargetType = "k8s"
	TargetType_TARGET_TYPE_DOCKER_COMPOSE TargetType = "docker-compose"
	TargetType_TARGET_TYPE_LOCAL          TargetType = "local"
)

type TargetSpec struct {
	Id            string     `json:"id,omitempty"`
	Type          TargetType `json:"type,omitempty"`
	HasLiveUpdate bool       `json:"hasLiveUpdate,omitempty"`
}

type BuildRecord struct {
	Error          string           `json:"error,omitempty"`
	Warnings       []string         `json:"warnings,omitempty"`
	StartTime      metav1.MicroTime `json:"startTime,omitempty"`
	FinishTime     metav1.MicroTime `json:"finishTime,omitempty"`
	IsCrashRebuild bool             `json:"isCrashRebuild,omitempty"`

	// The span id for this build record's logs in the main logstore.
	SpanId string `json:"spanId,omitempty"`
}

type K8SResourceInfo struct {
	PodName            string `json:"podName,omitempty"`
	PodCreationTime    string `json:"podCreationTime,omitempty"`
	PodUpdateStartTime string `json:"podUpdateStartTime,omitempty"`
	PodStatus          string `json:"podStatus,omitempty"`
	PodStatusMessage   string `json:"podStatusMessage,omitempty"`
	AllContainersReady bool   `json:"allContainersReady,omitempty"`
	PodRestarts        int32  `json:"podRestarts,omitempty"`

	// The span id for this pod's logs in the main logstore.
	SpanId       string   `json:"spanId,omitempty"`
	DisplayNames []string `json:"displayNames,omitempty"`
}

type LocalResourceInfo struct {
	Pid    int64 `json:"pid,omitempty"`
	IsTest bool  `json:"isTest,omitempty"`
}

type Link struct {
	Url  string `json:"url,omitempty"`
	Name string `json:"name,omitempty"`
}

type Resource struct {
	Name              string             `json:"name,omitempty"`
	LastDeployTime    metav1.MicroTime   `json:"lastDeployTime,omitempty"`
	TriggerMode       int32              `json:"triggerMode,omitempty"`
	BuildHistory      []*BuildRecord     `json:"buildHistory,omitempty"`
	CurrentBuild      *BuildRecord       `json:"currentBuild,omitempty"`
	PendingBuildSince metav1.MicroTime   `json:"pendingBuildSince,omitempty"`
	HasPendingChanges bool               `json:"hasPendingChanges,omitempty"`
	EndpointLinks     []*Link            `json:"endpointLinks,omitempty"`
	PodID             string             `json:"podID,omitempty"`
	K8SResourceInfo   *K8SResourceInfo   `json:"k8sResourceInfo,omitempty"`
	LocalResourceInfo *LocalResourceInfo `json:"localResourceInfo,omitempty"`
	RuntimeStatus     string             `json:"runtimeStatus,omitempty"`
	UpdateStatus      string             `json:"updateStatus,omitempty"`
	IsTiltfile        bool               `json:"isTiltfile,omitempty"`
	Specs             []*TargetSpec      `json:"specs,omitempty"`
	Queued            bool               `json:"queued,omitempty"`
}

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
	Resources            []*Resource      `json:"resources,omitempty"`
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
	UiSession   *v1alpha1.UISession    `json:"uiSession,omitempty"`
	UiResources []*v1alpha1.UIResource `json:"uiResources,omitempty"`
	UiButtons   []*v1alpha1.UIButton   `json:"uiButtons,omitempty"`
	Clusters    []*v1alpha1.Cluster    `json:"clusters,omitempty"`

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
