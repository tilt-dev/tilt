package view

import (
	"time"

	"github.com/windmilleng/tilt/internal/model"
)

type Resource struct {
	Name               string
	DirectoriesWatched []string
	PathsWatched       []string
	LastDeployTime     time.Time
	LastBuildEdits     []string

	LastBuildError      string
	LastBuildStartTime  time.Time
	LastBuildFinishTime time.Time
	LastBuildDuration   time.Duration
	LastBuildLog        string
	LastBuildReason     model.BuildReason

	PendingBuildReason model.BuildReason
	PendingBuildEdits  []string
	PendingBuildSince  time.Time

	// Maybe these fields should be combined into a BuildInfo struct, so that we
	// just have CurrentBuild, PendingBuild, LastBuild.
	CurrentBuildReason    model.BuildReason
	CurrentBuildEdits     []string
	CurrentBuildStartTime time.Time
	CurrentBuildLog       string

	PodName            string
	PodCreationTime    time.Time
	PodUpdateStartTime time.Time
	PodStatus          string
	PodRestarts        int
	Endpoints          []string
	PodLog             string

	// If a pod had to be killed because it was crashing, we keep the old log around
	// for a little while.
	CrashLog string

	IsYAMLManifest bool
}

func (r Resource) DefaultCollapse() bool {
	autoExpand := r.LastBuildError != "" ||
		r.CrashLog != "" ||
		r.PodRestarts > 0 ||
		r.PodStatus == "CrashLoopBackoff" ||
		r.PodStatus == "Error" ||
		r.LastBuildReason.Has(model.BuildReasonFlagCrash) ||
		r.CurrentBuildReason.Has(model.BuildReasonFlagCrash) ||
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
