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
type ViewState struct {
	ShowNarration    bool
	NarrationMessage string
	Resources        []ResourceViewState
}

type ResourceViewState struct {
	IsExpanded bool
}

type View struct {
	Resources []Resource
	ViewState ViewState
}
