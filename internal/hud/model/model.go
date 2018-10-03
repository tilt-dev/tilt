package model

import (
	"time"

	"github.com/windmilleng/tilt/internal/model"
)

type Pod struct {
	Name      string
	StartedAt time.Time
	Status    string
}

type Resource struct {
	DirectoryWatched   string
	LatestFileChanges  []string
	LastFileChangeTime time.Time
	Status             ResourceStatus
}

type ResourceStatus int

const (
	ResourceStatusUnbuilt ResourceStatus = iota
	ResourceStatusStale
	ResourceStatusFresh
)

func NewResource(directoryWatched string) Resource {
	return Resource{
		DirectoryWatched:   directoryWatched,
		LatestFileChanges:  []string{},
		LastFileChangeTime: time.Now(),
		Status:             ResourceStatusUnbuilt,
	}
}

type Model struct {
	Resources map[model.ManifestName]*Resource
	Pods      map[model.ManifestName]*Pod
}
