package view

import "time"

type Resource struct {
	Name               string
	DirectoriesWatched []string
	LastDeployTime     time.Time
	LastDeployEdits    []string

	LastBuildError      string
	LastBuildFinishTime time.Time
	LastBuildDuration   time.Duration
	LastBuildLog        string

	PendingBuildEdits []string
	PendingBuildSince time.Time

	CurrentBuildEdits     []string
	CurrentBuildStartTime time.Time

	PodName         string
	PodCreationTime time.Time
	PodStatus       string
	PodRestarts     int
	Endpoints       []string
	PodLog          string
}

// State of the current view that's not expressed in the underlying model state.
//
// This includes things like the current selection, warning messages,
// narration messages, etc.
//
// Client should always hold this as a value struct, and copy it
// whenever they need to mutate something.
type View struct {
	Log       string
	Resources []Resource
	ViewState ViewState
}

type ViewState struct {
	ShowNarration    bool
	NarrationMessage string
	Resources        []ResourceViewState
	LogModal         LogModal

	ProcessedLogByteCount int
}

type ResourceViewState struct {
	IsCollapsed bool
}

type LogModal struct {
	// if non-0, which resource's log is currently shown in a modal (1-based index)
	ResourceLogNumber int

	// if we're showing the full tilt log output in a modal
	TiltLog bool
}

func (lm LogModal) IsActive() bool {
	return lm.TiltLog || lm.ResourceLogNumber != 0
}
