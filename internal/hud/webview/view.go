package webview

import (
	"time"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockercompose"
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

type K8SResourceInfo struct {
	PodName            string
	PodCreationTime    time.Time
	PodUpdateStartTime time.Time
	PodStatus          string
	PodRestarts        int
	PodLog             model.Log
	YAML               string
}

var _ ResourceInfoView = K8SResourceInfo{}

func (K8SResourceInfo) resourceInfoView()             {}
func (k8sInfo K8SResourceInfo) RuntimeLog() model.Log { return k8sInfo.PodLog }
func (k8sInfo K8SResourceInfo) Status() string        { return k8sInfo.PodStatus }

type YAMLResourceInfo struct {
	K8sResources []string
}

var _ ResourceInfoView = YAMLResourceInfo{}

func (YAMLResourceInfo) resourceInfoView()              {}
func (yamlInfo YAMLResourceInfo) RuntimeLog() model.Log { return model.NewLog("") }
func (yamlInfo YAMLResourceInfo) Status() string        { return "" }

type Resource struct {
	Name               model.ManifestName
	DirectoriesWatched []string
	PathsWatched       []string
	LastDeployTime     time.Time

	BuildHistory []model.BuildRecord
	CurrentBuild model.BuildRecord

	PendingBuildReason model.BuildReason
	PendingBuildEdits  []string
	PendingBuildSince  time.Time

	Endpoints []string

	// TODO(nick): Remove ResourceInfoView. This is fundamentally a bad
	// data structure for the webview because the webview loses the Go type
	// on serialization to JS.
	ResourceInfo  ResourceInfoView
	RuntimeStatus RuntimeStatus

	// If a pod had to be killed because it was crashing, we keep the old log around
	// for a little while.
	CrashLog string

	IsTiltfile      bool
	ShowBuildStatus bool // if true, we show status & time in 'Build Status'; else, "N/A"
	CombinedLog     model.Log
}

func (r Resource) LastBuild() model.BuildRecord {
	if len(r.BuildHistory) == 0 {
		return model.BuildRecord{}
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
	Log                  model.Log
	Resources            []Resource
	TiltfileErrorMessage string
	LogTimestamps        bool
}
