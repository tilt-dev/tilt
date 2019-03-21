package webview

import (
	"time"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/model"
)

type ResourceInfoView interface {
	resourceInfoView()
	RuntimeLog() string
	Status() string
}

type DCResourceInfo struct {
	ConfigPath      string
	ContainerStatus dockercompose.Status
	ContainerID     container.ID
	Log             string
	StartTime       time.Time
}

func NewDCResourceInfo(configPath string, status dockercompose.Status, cID container.ID, log string, startTime time.Time) DCResourceInfo {
	return DCResourceInfo{
		ConfigPath:      configPath,
		ContainerStatus: status,
		ContainerID:     cID,
		Log:             log,
		StartTime:       startTime,
	}
}

var _ ResourceInfoView = DCResourceInfo{}

func (DCResourceInfo) resourceInfoView()         {}
func (dcInfo DCResourceInfo) RuntimeLog() string { return dcInfo.Log }
func (dcInfo DCResourceInfo) Status() string     { return string(dcInfo.ContainerStatus) }

type K8SResourceInfo struct {
	PodName            string
	PodCreationTime    time.Time
	PodUpdateStartTime time.Time
	PodStatus          string
	PodRestarts        int
	PodLog             string
	YAML               string
}

var _ ResourceInfoView = K8SResourceInfo{}

func (K8SResourceInfo) resourceInfoView()          {}
func (k8sInfo K8SResourceInfo) RuntimeLog() string { return k8sInfo.PodLog }
func (k8sInfo K8SResourceInfo) Status() string     { return k8sInfo.PodStatus }

type YAMLResourceInfo struct {
	K8sResources []string
}

var _ ResourceInfoView = YAMLResourceInfo{}

func (YAMLResourceInfo) resourceInfoView()           {}
func (yamlInfo YAMLResourceInfo) RuntimeLog() string { return "" }
func (yamlInfo YAMLResourceInfo) Status() string     { return "" }

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

	ResourceInfo ResourceInfoView

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

type View struct {
	Log                  string
	Resources            []Resource
	TiltfileErrorMessage string
	LogTimestamps        bool
}
