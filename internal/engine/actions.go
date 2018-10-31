package engine

import (
	"net/url"
	"time"

	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
	"k8s.io/api/core/v1"
)

type ErrorAction struct {
	Error error
}

func (ErrorAction) Action() {}

func NewErrorAction(err error) ErrorAction {
	return ErrorAction{Error: err}
}

type PodChangeAction struct {
	Pod *v1.Pod
}

func (PodChangeAction) Action() {}

func NewPodChangeAction(pod *v1.Pod) PodChangeAction {
	return PodChangeAction{Pod: pod}
}

type ServiceChangeAction struct {
	Service *v1.Service
	URL     *url.URL
}

func (ServiceChangeAction) Action() {}

func NewServiceChangeAction(service *v1.Service, url *url.URL) ServiceChangeAction {
	return ServiceChangeAction{Service: service, URL: url}
}

type PodLogAction struct {
	ManifestName model.ManifestName

	PodID k8s.PodID
	Log   []byte
}

func (PodLogAction) Action() {}

type BuildCompleteAction struct {
	Result store.BuildResult
	Error  error
}

func (BuildCompleteAction) Action() {}

func NewBuildCompleteAction(result store.BuildResult, err error) BuildCompleteAction {
	return BuildCompleteAction{
		Result: result,
		Error:  err,
	}
}

type InitAction struct {
	WatchMounts        bool
	Manifests          []model.Manifest
	GlobalYAMLManifest model.YAMLManifest
}

func (InitAction) Action() {}

type ManifestReloadedAction struct {
	OldManifest model.Manifest
	NewManifest model.Manifest
	Error       *manifestErr
}

func (ManifestReloadedAction) Action() {}

type BuildStartedAction struct {
	Manifest     model.Manifest
	StartTime    time.Time
	FilesChanged []string
}

func (BuildStartedAction) Action() {}

type GlobalYAMLManifestReloadedAction struct {
	GlobalYAML model.YAMLManifest
}

func (GlobalYAMLManifestReloadedAction) Action() {}
