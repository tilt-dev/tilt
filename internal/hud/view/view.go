package view

import (
	"reflect"
	"time"

	"github.com/windmilleng/tilt/internal/model"
)

type ResourceInfoView interface {
	resourceInfoView()
	RuntimeLog() string
	Status() string
}

type DCResourceInfo struct {
	ConfigPath string
	status     string
	Log        string
}

func NewDCResourceInfo(configPath string, status string, log string) DCResourceInfo {
	return DCResourceInfo{
		ConfigPath: configPath,
		status:     status,
		Log:        log,
	}
}

var _ ResourceInfoView = DCResourceInfo{}

func (DCResourceInfo) resourceInfoView()     {}
func (dc DCResourceInfo) Empty() bool        { return reflect.DeepEqual(dc, DCResourceInfo{}) }
func (dc DCResourceInfo) RuntimeLog() string { return dc.Log }
func (dc DCResourceInfo) Status() string     { return dc.status }

type K8SResourceInfo struct {
	PodName            string
	PodCreationTime    time.Time
	PodUpdateStartTime time.Time
	PodStatus          string
	PodRestarts        int
	PodLog             string
}

var _ ResourceInfoView = K8SResourceInfo{}

func (K8SResourceInfo) resourceInfoView()      {}
func (kri K8SResourceInfo) RuntimeLog() string { return kri.PodLog }
func (kri K8SResourceInfo) Status() string     { return kri.PodStatus }

type YAMLResourceInfo struct {
	K8sResources []string
}

var _ ResourceInfoView = YAMLResourceInfo{}

func (YAMLResourceInfo) resourceInfoView()      {}
func (yri YAMLResourceInfo) RuntimeLog() string { return "" }
func (yri YAMLResourceInfo) Status() string     { return "" }

type Resource struct {
	Name               model.ManifestName
	DirectoriesWatched []string
	PathsWatched       []string
	LastDeployTime     time.Time

	BuildHistory []model.BuildStatus
	CurrentBuild model.BuildStatus

	PendingBuildReason model.BuildReason
	PendingBuildEdits  []string
	PendingBuildSince  time.Time

	Endpoints []string

	ResourceInfo ResourceInfoView

	// If a pod had to be killed because it was crashing, we keep the old log around
	// for a little while.
	CrashLog string

	IsYAMLManifest bool
}

func (r Resource) DCInfo() DCResourceInfo {
	switch info := r.ResourceInfo.(type) {
	case DCResourceInfo:
		return info
	default:
		return DCResourceInfo{}
	}
}

func (r Resource) IsDC() bool {
	return !r.DCInfo().Empty()
}

func (r Resource) K8SInfo() K8SResourceInfo {
	ret, _ := r.ResourceInfo.(K8SResourceInfo)
	return ret
}

func (r Resource) IsK8S() bool {
	_, ok := r.ResourceInfo.(K8SResourceInfo)
	return ok
}

func (r Resource) YamlInfo() YAMLResourceInfo {
	switch info := r.ResourceInfo.(type) {
	case YAMLResourceInfo:
		return info
	default:
		return YAMLResourceInfo{}
	}
}

func (r Resource) LastBuild() model.BuildStatus {
	if len(r.BuildHistory) == 0 {
		return model.BuildStatus{}
	}
	return r.BuildHistory[0]
}

func (r Resource) DefaultCollapse() bool {
	autoExpand := false
	if r.IsK8S() {
		kri := r.K8SInfo()
		autoExpand = kri.PodRestarts > 0 || kri.PodStatus == "CrashLoopBackoff" || kri.PodStatus == "Error"
	}
	autoExpand = autoExpand ||
		r.LastBuild().Error != nil ||
		r.CrashLog != "" ||
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
	Log                  string
	Resources            []Resource
	TiltfileErrorMessage string
	TriggerMode          model.TriggerMode
}

type ViewState struct {
	ShowNarration         bool
	NarrationMessage      string
	Resources             []ResourceViewState
	LogModal              LogModal
	ProcessedLogByteCount int
	AlertMessage          string
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

func (vs *ViewState) CycleViewLogState() {
	states := []TiltLogState{TiltLogPane, TiltLogFullScreen, TiltLogMinimized}
	for i := range states {
		if states[i] == vs.LogModal.TiltLog {
			vs.LogModal.TiltLog = states[(i+1)%len(states)]
			return
		}
	}
	vs.LogModal.TiltLog = TiltLogFullScreen
}

type ResourceViewState struct {
	CollapseState CollapseState
}

type LogModal struct {
	// if non-0, which resource's log is currently shown in a modal (1-based index)
	ResourceLogNumber int

	// if we're showing the full tilt log output in a modal
	TiltLog TiltLogState
}

type TiltLogState int

const (
	TiltLogPane TiltLogState = iota
	TiltLogFullScreen
	TiltLogMinimized
)
