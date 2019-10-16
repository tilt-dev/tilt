package webview

import (
	"time"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/pkg/model"
)

type ResourceInfoView interface {
	resourceInfoView()
	RuntimeLog() model.Log
	Status() string
}

type DCResourceInfo struct {
	ConfigPaths     []string
	ContainerStatus dockercompose.Status
	ContainerID     container.ID
	Log             model.Log
	StartTime       time.Time
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

var _ ResourceInfoView = DCResourceInfo{}

func (DCResourceInfo) resourceInfoView()            {}
func (dcInfo DCResourceInfo) RuntimeLog() model.Log { return dcInfo.Log }
func (dcInfo DCResourceInfo) Status() string        { return string(dcInfo.ContainerStatus) }

type K8sResourceInfo struct {
	PodName            string
	PodCreationTime    time.Time
	PodUpdateStartTime time.Time
	PodStatus          string
	PodStatusMessage   string
	AllContainersReady bool
	PodRestarts        int
	PodLog             model.Log
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
	K8sResources []string
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
	Edits          []string
	Error          error
	Warnings       []string
	StartTime      time.Time
	FinishTime     time.Time // IsZero() == true for in-progress builds
	Log            model.Log
	IsCrashRebuild bool
}

func ToWebViewBuildRecord(br model.BuildRecord) BuildRecord {
	return BuildRecord{
		Edits:          br.Edits,
		Error:          br.Error,
		Warnings:       br.Warnings,
		StartTime:      br.StartTime,
		FinishTime:     br.FinishTime,
		Log:            br.Log,
		IsCrashRebuild: br.Reason.IsCrashOnly(),
	}
}

func ToWebViewBuildRecords(brs []model.BuildRecord) []BuildRecord {
	ret := make([]BuildRecord, len(brs))
	for i, br := range brs {
		ret[i] = ToWebViewBuildRecord(br)
	}
	return ret
}

type Resource struct {
	Name               model.ManifestName
	DirectoriesWatched []string
	PathsWatched       []string
	LastDeployTime     time.Time
	TriggerMode        model.TriggerMode

	BuildHistory []BuildRecord
	CurrentBuild BuildRecord

	PendingBuildReason model.BuildReason
	PendingBuildEdits  []string
	PendingBuildSince  time.Time
	HasPendingChanges  bool

	Endpoints []string
	PodID     k8s.PodID

	// Only one of these resource info fields will be populated
	K8sResourceInfo   *K8sResourceInfo   `json:",omitempty"`
	DCResourceInfo    *DCResourceInfo    `json:",omitempty"`
	YAMLResourceInfo  *YAMLResourceInfo  `json:",omitempty"`
	LocalResourceInfo *LocalResourceInfo `json:",omitempty"`

	RuntimeStatus RuntimeStatus

	IsTiltfile      bool
	ShowBuildStatus bool // if true, we show status & time in 'Build Status'; else, "N/A"
	CombinedLog     model.Log
	CrashLog        model.Log
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

type View struct {
	Log           model.Log
	Resources     []Resource
	LogTimestamps bool

	FeatureFlags map[string]bool

	NeedsAnalyticsNudge bool

	RunningTiltBuild model.TiltBuild
	LatestTiltBuild  model.TiltBuild

	TiltCloudUsername   string
	TiltCloudSchemeHost string
	TiltCloudTeamID     string

	FatalError string
}

func (v View) Resource(n model.ManifestName) (Resource, bool) {
	for _, res := range v.Resources {
		if res.Name == n {
			return res, true
		}
	}
	return Resource{}, false
}
