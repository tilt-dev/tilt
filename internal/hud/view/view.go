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

	PendingBuildEdits []string
	PendingBuildSince time.Time

	CurrentBuildEdits     []string
	CurrentBuildStartTime time.Time

	PodName         string
	PodCreationTime time.Time
	PodStatus       string
	PodRestarts     int
	Endpoints       []string
}

type View struct {
	Resources []Resource
}
