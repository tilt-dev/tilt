package view

import (
	"time"

	hudmodel "github.com/windmilleng/tilt/internal/hud/model"
	tiltmodel "github.com/windmilleng/tilt/internal/model"
)

func NewView(m hudmodel.Model) View {
	var resources []Resource = nil
	for name := range m.Resources {
		resources = append(resources, NewResourceView(m, name, *m.Resources[name]))
	}

	return View{resources}
}

func NewResourceView(m hudmodel.Model, name tiltmodel.ManifestName, r hudmodel.Resource) Resource {
	ret := Resource{
		Name:                    name.String(),
		DirectoryWatched:        r.DirectoryWatched,
		LatestFileChanges:       r.LatestFileChanges,
		TimeSinceLastFileChange: time.Now().Sub(r.LastFileChangeTime),
		Status:                  ResourceStatusStale,
		StatusDesc:              "No pod found",
	}

	pod, ok := m.Pods[name]
	if ok {
		// TODO(matt) this mapping is probably wrong
		switch pod.Status {
		case "Running":
			ret.Status = ResourceStatusFresh
		case "Pending":
			ret.Status = ResourceStatusStale
		default:
			ret.Status = ResourceStatusBroken
		}
		ret.StatusDesc = pod.Status
	}
	return ret
}
