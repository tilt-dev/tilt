package view

import (
	"time"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/model"
)

const TiltfileResourceName = "(Tiltfile)"

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

	ResourceInfo ResourceInfoView

	// If a pod had to be killed because it was crashing, we keep the old log around
	// for a little while.
	CrashLog model.Log

	IsTiltfile bool
}

func (r Resource) DockerComposeTarget() DCResourceInfo {
	switch info := r.ResourceInfo.(type) {
	case DCResourceInfo:
		return info
	default:
		return DCResourceInfo{}
	}
}

func (r Resource) DCInfo() DCResourceInfo {
	ret, _ := r.ResourceInfo.(DCResourceInfo)
	return ret
}

func (r Resource) IsDC() bool {
	_, ok := r.ResourceInfo.(DCResourceInfo)
	return ok
}

func (r Resource) K8SInfo() K8SResourceInfo {
	ret, _ := r.ResourceInfo.(K8SResourceInfo)
	return ret
}

func (r Resource) IsK8S() bool {
	_, ok := r.ResourceInfo.(K8SResourceInfo)
	return ok
}

func (r Resource) YAMLInfo() YAMLResourceInfo {
	ret, _ := r.ResourceInfo.(YAMLResourceInfo)
	return ret
}

func (r Resource) IsYAML() bool {
	_, ok := r.ResourceInfo.(YAMLResourceInfo)
	return ok
}

func (r Resource) LastBuild() model.BuildRecord {
	if len(r.BuildHistory) == 0 {
		return model.BuildRecord{}
	}
	return r.BuildHistory[0]
}

func (r Resource) DefaultCollapse() bool {
	autoExpand := false
	if k8sInfo, ok := r.ResourceInfo.(K8SResourceInfo); ok {
		autoExpand = k8sInfo.PodRestarts > 0 || k8sInfo.PodStatus == "CrashLoopBackoff" || k8sInfo.PodStatus == "Error"
	}

	if r.IsYAML() {
		autoExpand = true
	}

	if r.IsDC() && r.DockerComposeTarget().Status() == string(dockercompose.StatusCrash) {
		autoExpand = true
	}

	autoExpand = autoExpand ||
		r.LastBuild().Error != nil ||
		!r.CrashLog.Empty() ||
		r.LastBuild().Reason.Has(model.BuildReasonFlagCrash) ||
		r.CurrentBuild.Reason.Has(model.BuildReasonFlagCrash) ||
		r.PendingBuildReason.Has(model.BuildReasonFlagCrash)
	return !autoExpand
}

func (r Resource) IsCollapsed(rv ResourceViewState) bool {
	return rv.CollapseState.IsCollapsed(r.DefaultCollapse())
}

// State of the current view that's not expressed in the underlying model state.
//
// This includes things like the current selection, warning messages,
// narration messages, etc.
//
// Client should always hold this as a value struct, and copy it
// whenever they need to mutate something.
type View struct {
	Log                  model.Log
	Resources            []Resource
	TiltfileErrorMessage string
	TriggerMode          model.TriggerMode
	IsProfiling          bool
	LogTimestamps        bool
}

type ViewState struct {
	ShowNarration         bool
	NarrationMessage      string
	Resources             []ResourceViewState
	ProcessedLogByteCount int
	AlertMessage          string
	SelectedIndex         int
}

type CollapseState int

const (
	CollapseAuto = iota
	CollapseYes
	CollapseNo
)

func (c CollapseState) IsCollapsed(defaultCollapse bool) bool {
	switch c {
	case CollapseYes:
		return true
	case CollapseNo:
		return false
	default: // CollapseAuto
		return defaultCollapse
	}
}

type ResourceViewState struct {
	CollapseState CollapseState
}
