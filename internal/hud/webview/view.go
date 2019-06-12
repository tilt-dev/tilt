package webview

import (
	"time"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
)

type ResourceInfoView interface {
	resourceInfoView()
	RuntimeLog() model.Log
	Status() string
}

type DCResourceInfo struct {
	ConfigPath      string
	ContainerStatus dockercompose.Status
	ContainerID     container.ID
	Log             model.Log
	StartTime       time.Time
}

func NewDCResourceInfo(configPath string, status dockercompose.Status, cID container.ID, log model.Log, startTime time.Time) DCResourceInfo {
	return DCResourceInfo{
		ConfigPath:      configPath,
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
	PodRestarts        int
	PodLog             model.Log
	YAML               string
}

var _ ResourceInfoView = K8sResourceInfo{}

func (K8sResourceInfo) resourceInfoView()             {}
func (k8sInfo K8sResourceInfo) RuntimeLog() model.Log { return k8sInfo.PodLog }
func (k8sInfo K8sResourceInfo) Status() string        { return k8sInfo.PodStatus }

type YAMLResourceInfo struct {
	K8sResources []string
}

var _ ResourceInfoView = YAMLResourceInfo{}

func (YAMLResourceInfo) resourceInfoView()              {}
func (yamlInfo YAMLResourceInfo) RuntimeLog() model.Log { return model.NewLog("") }
func (yamlInfo YAMLResourceInfo) Status() string        { return "" }

type BuildRecord struct {
	model.BuildRecord
	IsCrashRebuild bool
}

func ToWebViewBuildRecord(br model.BuildRecord) BuildRecord {
	return BuildRecord{
		BuildRecord:    br,
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

	// TODO(nick): Remove ResourceInfoView. This is fundamentally a bad
	// data structure for the webview because the webview loses the Go type
	// on serialization to JS.
	ResourceInfo  ResourceInfoView
	RuntimeStatus RuntimeStatus

	IsTiltfile      bool
	ShowBuildStatus bool // if true, we show status & time in 'Build Status'; else, "N/A"
	CombinedLog     model.Log
	CrashLog        model.Log
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
	RuntimeStatusPending               = "pending"
	RuntimeStatusError                 = "error"
)

type View struct {
	Log           model.Log
	Resources     []Resource
	LogTimestamps bool

	SailEnabled bool
	SailURL     string

	NeedsAnalyticsNudge bool

	RunningTiltBuild model.TiltBuild
	LatestTiltBuild  model.TiltBuild
}

func (v View) Resource(n model.ManifestName) (Resource, bool) {
	for _, res := range v.Resources {
		if res.Name == n {
			return res, true
		}
	}
	return Resource{}, false
}
