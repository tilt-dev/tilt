package engine

import (
	"time"

	"github.com/windmilleng/tilt/internal/hud/hudview"

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
	Status             resourceStatus
}

type resourceStatus int

const (
	resourceStatusUnbuilt resourceStatus = iota
	resourceStatusStale
	resourceStatusFresh
)

func newResource(directoryWatched string) Resource {
	return Resource{
		DirectoryWatched:   directoryWatched,
		LatestFileChanges:  []string{},
		LastFileChangeTime: time.Now(),
		Status:             resourceStatusUnbuilt,
	}
}

type upperState struct {
	Resources map[model.ManifestName]*Resource
	Pods      map[model.ManifestName]*Pod
}

func newView(us upperState) hudview.View {
	var resources []hudview.Resource
	for name, r := range us.Resources {
		resources = append(resources, NewResourceView(us, name, *r))
	}

	return hudview.View{Resources: resources}
}

func NewResourceView(us upperState, name model.ManifestName, r Resource) hudview.Resource {
	ret := hudview.Resource{
		Name:                    name.String(),
		DirectoryWatched:        r.DirectoryWatched,
		LatestFileChanges:       r.LatestFileChanges,
		TimeSinceLastFileChange: time.Now().Sub(r.LastFileChangeTime),
		Status:                  hudview.ResourceStatusStale,
		StatusDesc:              "No pod found",
	}

	if pod, ok := us.Pods[name]; ok {
		// TODO(matt) this mapping is probably wrong
		switch pod.Status {
		case "Running":
			ret.Status = hudview.ResourceStatusFresh
		case "Pending":
			ret.Status = hudview.ResourceStatusStale
		default:
			ret.Status = hudview.ResourceStatusBroken
		}
		ret.StatusDesc = pod.Status
	}
	return ret
}
