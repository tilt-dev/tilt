package view

import "time"

type Resource struct {
	Name             string
	DirectoryWatched string
	LastDeployTime   time.Time
	LastDeployEdits  []string

	LastBuildError      string
	LastBuildFinishTime time.Time
	LastBuildDuration   time.Duration

	PendingBuildEdits []string
	PendingBuildSince time.Time

	CurrentBuildEdits     []string
	CurrentBuildStartTime time.Time

	PodName         string
	PodCreationTime time.Time
	PodStatus       string
	Endpoint        string
}

type View struct {
	Resources []Resource
}
