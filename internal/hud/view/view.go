package view

import (
	"time"

	"github.com/windmilleng/tilt/internal/container"
	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/pkg/model"
	"github.com/windmilleng/tilt/pkg/model/logstore"
)

const LogLineCount = 50

const TiltfileResourceName = "(Tiltfile)"

type ResourceInfoView interface {
	resourceInfoView()
	RuntimeSpanID() logstore.SpanID
	Status() string
}

type DCResourceInfo struct {
	ConfigPaths     []string
	ContainerStatus dockercompose.Status
	ContainerID     container.ID
	SpanID          logstore.SpanID
	StartTime       time.Time
}

func NewDCResourceInfo(configPaths []string, status dockercompose.Status, cID container.ID, spanID logstore.SpanID, startTime time.Time) DCResourceInfo {
	return DCResourceInfo{
		ConfigPaths:     configPaths,
		ContainerStatus: status,
		ContainerID:     cID,
		SpanID:          spanID,
		StartTime:       startTime,
	}
}

var _ ResourceInfoView = DCResourceInfo{}

func (DCResourceInfo) resourceInfoView()                     {}
func (dcInfo DCResourceInfo) RuntimeSpanID() logstore.SpanID { return dcInfo.SpanID }
func (dcInfo DCResourceInfo) Status() string                 { return string(dcInfo.ContainerStatus) }

type K8sResourceInfo struct {
	PodName            string
	PodCreationTime    time.Time
	PodUpdateStartTime time.Time
	PodStatus          string
	PodRestarts        int
	SpanID             logstore.SpanID
}

var _ ResourceInfoView = K8sResourceInfo{}

func (K8sResourceInfo) resourceInfoView()                      {}
func (k8sInfo K8sResourceInfo) RuntimeSpanID() logstore.SpanID { return k8sInfo.SpanID }
func (k8sInfo K8sResourceInfo) Status() string                 { return k8sInfo.PodStatus }

type YAMLResourceInfo struct {
	K8sResources []string
}

var _ ResourceInfoView = YAMLResourceInfo{}

func (YAMLResourceInfo) resourceInfoView()                       {}
func (yamlInfo YAMLResourceInfo) RuntimeSpanID() logstore.SpanID { return "unknown" }
func (yamlInfo YAMLResourceInfo) Status() string                 { return "" }

type LocalResourceInfo struct {
	status model.RuntimeStatus
	pid    int
	spanID model.LogSpanID
}

func NewLocalResourceInfo(status model.RuntimeStatus, pid int, spanID model.LogSpanID) LocalResourceInfo {
	return LocalResourceInfo{status: status, pid: pid, spanID: spanID}
}

var _ ResourceInfoView = LocalResourceInfo{}

func (lri LocalResourceInfo) resourceInfoView()              {}
func (lri LocalResourceInfo) RuntimeSpanID() logstore.SpanID { return lri.spanID }
func (lri LocalResourceInfo) Status() string                 { return string(lri.status) }

type Resource struct {
	Name               model.ManifestName
	DirectoriesWatched []string
	PathsWatched       []string
	LastDeployTime     time.Time
	TriggerMode        model.TriggerMode

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

func (r Resource) K8sInfo() K8sResourceInfo {
	ret, _ := r.ResourceInfo.(K8sResourceInfo)
	return ret
}

func (r Resource) IsK8s() bool {
	_, ok := r.ResourceInfo.(K8sResourceInfo)
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
	if k8sInfo, ok := r.ResourceInfo.(K8sResourceInfo); ok {
		autoExpand = k8sInfo.PodRestarts > 0 || k8sInfo.PodStatus == "CrashLoopBackOff" || k8sInfo.PodStatus == "Error"
	}

	if r.IsDC() && r.DockerComposeTarget().Status() == string(dockercompose.StatusCrash) {
		autoExpand = true
	}

	autoExpand = autoExpand ||
		r.LastBuild().Error != nil ||
		!r.CrashLog.Empty() ||
		r.LastBuild().WarningCount > 0 ||
		r.LastBuild().Reason.Has(model.BuildReasonFlagCrash) ||
		r.CurrentBuild.Reason.Has(model.BuildReasonFlagCrash) ||
		r.PendingBuildReason.Has(model.BuildReasonFlagCrash)
	return !autoExpand
}

func (r Resource) IsCollapsed(rv ResourceViewState) bool {
	return rv.CollapseState.IsCollapsed(r.DefaultCollapse())
}

// Snapshot of the current view that's not expressed in the underlying model state.
//
// This includes things like the current selection, warning messages,
// narration messages, etc.
//
// Client should always hold this as a value struct, and copy it
// whenever they need to mutate something.
type View struct {
	LogReader   logstore.Reader
	Resources   []Resource
	IsProfiling bool
	FatalError  error
}

func (v View) TiltfileErrorMessage() string {
	for _, res := range v.Resources {
		if res.Name == TiltfileResourceName {
			err := res.LastBuild().Error
			if err != nil {
				return err.Error()
			}
			return ""
		}
	}
	return ""
}

func (v View) Resource(n model.ManifestName) (Resource, bool) {
	for _, res := range v.Resources {
		if res.Name == n {
			return res, true
		}
	}
	return Resource{}, false
}

type ViewState struct {
	ShowNarration    bool
	NarrationMessage string
	Resources        []ResourceViewState
	ProcessedLogs    logstore.Checkpoint
	AlertMessage     string
	TabState         TabState
	SelectedIndex    int
	TiltLogState     TiltLogState
}

type TabState int

const (
	TabAllLog TabState = iota
	TabBuildLog
	TabRuntimeLog
)

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

func (vs *ViewState) CycleViewLogState() {
	states := []TiltLogState{TiltLogShort, TiltLogHalfScreen, TiltLogFullScreen}
	for i := range states {
		if states[i] == vs.TiltLogState {
			vs.TiltLogState = states[(i+1)%len(states)]
			return
		}
	}
	vs.TiltLogState = TiltLogFullScreen
}

type TiltLogState int

const (
	TiltLogShort TiltLogState = iota
	TiltLogHalfScreen
	TiltLogFullScreen
)

type ResourceViewState struct {
	CollapseState CollapseState
}
