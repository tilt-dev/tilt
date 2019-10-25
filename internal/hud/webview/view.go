package webview

import (
	"time"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/pkg/model"

	"github.com/golang/protobuf/ptypes"
	proto_webview "github.com/windmilleng/tilt/pkg/webview"
	"github.com/golang/protobuf/ptypes/timestamp"
)

type ResourceInfoView interface {
	resourceInfoView()
	RuntimeLog() model.Log
	Status() string
}

type DCResourceInfo struct {
	ConfigPaths     []string             `json:"configPaths"`
	ContainerStatus dockercompose.Status `json:"containerStatus"`
	ContainerID     container.ID         `json:"containerID"`
	Log             model.Log            `json:"log"`
	StartTime       time.Time            `json:"startTime"`
}

func NewDCResourceInfo(configPaths []string, status dockercompose.Status, cID container.ID, log model.Log, startTime time.Time) DCResourceInfo {
	return DCResourceInfo{
		ConfigPaths:     configPaths,
		ContainerStatus: status,
		ContainerID:     cID,
		Log:             log,
		StartTime:       startTime,
	}
}

func NewProtoDCResourceInfo(configPaths []string, status dockercompose.Status, cID container.ID, log model.Log, startTime time.Time) *proto_webview.DCResourceInfo {
	return &proto_webview.DCResourceInfo{
		ConfigPaths:     configPaths,
		ContainerStatus: string(status),
		ContainerID:     string(cID),
		Log:             log.String(),
		StartTime:       timeToProto(startTime),
	}
}

var _ ResourceInfoView = DCResourceInfo{}

func (DCResourceInfo) resourceInfoView()            {}
func (dcInfo DCResourceInfo) RuntimeLog() model.Log { return dcInfo.Log }
func (dcInfo DCResourceInfo) Status() string        { return string(dcInfo.ContainerStatus) }

type K8sResourceInfo struct {
	PodName            string    `json:"podName"`
	PodCreationTime    time.Time `json:"podCreationTime"`
	PodUpdateStartTime time.Time `json:"podUpdateStartTime"`
	PodStatus          string    `json:"podStatus"`
	PodStatusMessage   string    `json:"podStatusMessage"`
	AllContainersReady bool      `json:"allContainersReady"`
	PodRestarts        int       `json:"podRestarts"`
	PodLog             model.Log `json:"podLog"`
}

var _ ResourceInfoView = K8sResourceInfo{}

func (K8sResourceInfo) resourceInfoView()             {}
func (k8sInfo K8sResourceInfo) RuntimeLog() model.Log { return k8sInfo.PodLog }
func (k8sInfo K8sResourceInfo) Status() string {
	status := k8sInfo.PodStatus
	if status == "Running" && !k8sInfo.AllContainersReady {
		status = "Pending"
	}
	return status
}

type YAMLResourceInfo struct {
	K8sResources []string `json:"k8sResources"`
}

var _ ResourceInfoView = YAMLResourceInfo{}

func (YAMLResourceInfo) resourceInfoView()              {}
func (yamlInfo YAMLResourceInfo) RuntimeLog() model.Log { return model.NewLog("") }
func (yamlInfo YAMLResourceInfo) Status() string        { return "" }

// Local resources have no run time info, so it's all empty.
type LocalResourceInfo struct{}

var _ ResourceInfoView = LocalResourceInfo{}

func (LocalResourceInfo) resourceInfoView()     {}
func (LocalResourceInfo) RuntimeLog() model.Log { return model.NewLog("") }
func (LocalResourceInfo) Status() string        { return "" }

type BuildRecord struct {
	Edits          []string  `json:"edits"`
	Error          string    `json:"error"`
	Warnings       []string  `json:"warnings"`
	StartTime      time.Time `json:"startTime"`
	FinishTime     time.Time `json:"finishTime"` // IsZero() == true for in-progress builds
	Log            model.Log `json:"log"`
	IsCrashRebuild bool      `json:"isCrashRebuild"`
}

func ToWebViewBuildRecord(br model.BuildRecord) BuildRecord {
	e := ""
	if br.Error != nil {
		e = br.Error.Error()
	}
	return BuildRecord{
		Edits:          br.Edits,
		Error:          e,
		Warnings:       br.Warnings,
		StartTime:      br.StartTime,
		FinishTime:     br.FinishTime,
		Log:            br.Log,
		IsCrashRebuild: br.Reason.IsCrashOnly(),
	}
}

func timeToProto(t time.Time) *timestamp.Timestamp {
	ts, err := ptypes.TimestampProto(t)
	if err != nil {
		return nil
	}

	return ts
}

func ToProtoBuildRecord(br model.BuildRecord) *proto_webview.BuildRecord {
	e := ""
	if br.Error != nil {
		e = br.Error.Error()
	}

	return &proto_webview.BuildRecord{
		Edits:          br.Edits,
		Error:          e,
		Warnings:       br.Warnings,
		StartTime:      timeToProto(br.StartTime),
		FinishTime:     timeToProto(br.FinishTime),
		Log:            br.Log.String(),
		IsCrashRebuild: br.Reason.IsCrashOnly(),
	}
}

func ToProtoBuildRecords(brs []model.BuildRecord) []*proto_webview.BuildRecord {
	ret := make([]*proto_webview.BuildRecord, len(brs))
	for i, br := range brs {
		ret[i] = ToProtoBuildRecord(br)
	}
	return ret
}

func ToWebViewBuildRecords(brs []model.BuildRecord) []BuildRecord {
	ret := make([]BuildRecord, len(brs))
	for i, br := range brs {
		ret[i] = ToWebViewBuildRecord(br)
	}
	return ret
}

type Alert struct {
	AlertType    string `json:"alertType"`
	Header       string `json:"header"`
	Message      string `json:"msg"`
	Timestamp    string `json:"timestamp"`
	ResourceName string `json:"resourceName"`
}

type Resource struct {
	Name               model.ManifestName `json:"name"`
	DirectoriesWatched []string           `json:"directoriesWatched"`
	PathsWatched       []string           `json:"pathsWatched"`
	LastDeployTime     time.Time          `json:"lastDeployTime"`
	TriggerMode        model.TriggerMode  `json:"triggerMode"`

	BuildHistory []BuildRecord `json:"buildHistory"`
	CurrentBuild BuildRecord   `json:"currentBuild"`

	PendingBuildReason model.BuildReason `json:"pendingBuildReason"`
	PendingBuildEdits  []string          `json:"pendingBuildEdits"`
	PendingBuildSince  time.Time         `json:"pendingBuildSince"`
	HasPendingChanges  bool              `json:"hasPendingChanges"`

	Endpoints []string  `json:"endpoints"`
	PodID     k8s.PodID `json:"podID"`

	// Only one of these resource info fields will be populated
	K8sResourceInfo   *K8sResourceInfo   `json:"k8sResourceInfo,omitempty"`
	DCResourceInfo    *DCResourceInfo    `json:"dcResourceInfo,omitempty"`
	YAMLResourceInfo  *YAMLResourceInfo  `json:"yamlResourceInfo,omitempty"`
	LocalResourceInfo *LocalResourceInfo `json:"localResourceInfo,omitempty"`

	RuntimeStatus RuntimeStatus `json:"runtimeStatus"`

	IsTiltfile      bool      `json:"isTiltfile"`
	ShowBuildStatus bool      `json:"showBuildStatus"` // if true, we show status & time in 'Build Status'; else, "N/A"
	CombinedLog     model.Log `json:"combinedLog"`
	CrashLog        model.Log `json:"crashLog"`

	Alerts []Alert `json:"alerts"`

	Facets []model.Facet `json:"facets"`
}

func (r Resource) ResourceInfo() ResourceInfoView {
	if r.K8sResourceInfo != nil {
		return *r.K8sResourceInfo
	}
	if r.DCResourceInfo != nil {
		return *r.DCResourceInfo
	}
	if r.YAMLResourceInfo != nil {
		return *r.YAMLResourceInfo
	}
	if r.LocalResourceInfo != nil {
		return *r.LocalResourceInfo
	}

	// return an empty info, just to avoid NPEs
	return K8sResourceInfo{}
}

func (r Resource) LastBuild() BuildRecord {
	if len(r.BuildHistory) == 0 {
		return BuildRecord{}
	}
	return r.BuildHistory[0]
}

type RuntimeStatus string

const (
	RuntimeStatusOK      RuntimeStatus = "ok"
	RuntimeStatusPending RuntimeStatus = "pending"
	RuntimeStatusError   RuntimeStatus = "error"
)

type TiltBuild struct {
	Version   string `json:"version"`
	CommitSHA string `json:"commitSHA"`
	Date      string `json:"date"`
	Dev       bool   `json:"dev"`
}

type View struct {
	Log           model.Log  `json:"log"`
	Resources     []Resource `json:"resources"`
	LogTimestamps bool       `json:"logTimestamps"`

	FeatureFlags map[string]bool `json:"featureFlags"`

	NeedsAnalyticsNudge bool `json:"needsAnalyticsNudge"`

	RunningTiltBuild TiltBuild `json:"runningTiltBuild"`
	LatestTiltBuild  TiltBuild `json:"latestTiltBuild"`

	TiltCloudUsername   string `json:"tiltCloudUsername"`
	TiltCloudSchemeHost string `json:"tiltCloudSchemeHost"`
	TiltCloudTeamID     string `json:"tiltCloudTeamID"`

	FatalError string `json:"fatalError"`
}

func (v View) Resource(n model.ManifestName) (Resource, bool) {
	for _, res := range v.Resources {
		if res.Name == n {
			return res, true
		}
	}
	return Resource{}, false
}
