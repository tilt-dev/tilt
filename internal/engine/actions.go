package engine

import (
	"net/url"
	"time"

	"github.com/windmilleng/tilt/internal/dockercompose"
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

type BuildLogAction struct {
	ManifestName model.ManifestName
	Log          []byte
}

func (BuildLogAction) Action() {}

type PodLogAction struct {
	ManifestName model.ManifestName

	PodID k8s.PodID
	Log   []byte
}

func (PodLogAction) Action() {}

type LogAction struct {
	Log []byte
}

func (LogAction) Action() {}

type BuildCompleteAction struct {
	Result store.BuildResultSet
	Error  error
}

func (BuildCompleteAction) Action() {}

func NewBuildCompleteAction(result store.BuildResultSet, err error) BuildCompleteAction {
	return BuildCompleteAction{
		Result: result,
		Error:  err,
	}
}

type InitAction struct {
	WatchMounts        bool
	Manifests          []model.Manifest
	GlobalYAMLManifest model.YAMLManifest
	TiltfilePath       string
	ConfigFiles        []string
	InitManifests      []model.ManifestName
	TriggerMode        model.TriggerMode

	StartTime  time.Time
	FinishTime time.Time
	Err        error
}

func (InitAction) Action() {}

type ManifestReloadedAction struct {
	OldManifest model.Manifest
	NewManifest model.Manifest
	Error       error
}

func (ManifestReloadedAction) Action() {}

type BuildStartedAction struct {
	ManifestName model.ManifestName
	StartTime    time.Time
	FilesChanged []string
	Reason       model.BuildReason
}

func (BuildStartedAction) Action() {}

type GlobalYAMLApplyStartedAction struct{}

func (GlobalYAMLApplyStartedAction) Action() {}

type GlobalYAMLApplyCompleteAction struct {
	Error error
}

func (GlobalYAMLApplyCompleteAction) Action() {}

type HudStoppedAction struct {
	err error
}

func (HudStoppedAction) Action() {}

func NewHudStoppedAction(err error) HudStoppedAction {
	return HudStoppedAction{err}
}

type ConfigsReloadStartedAction struct {
	FilesChanged map[string]bool
}

func (ConfigsReloadStartedAction) Action() {}

type ConfigsReloadedAction struct {
	Manifests   []model.Manifest
	GlobalYAML  model.YAMLManifest
	ConfigFiles []string

	StartTime  time.Time
	FinishTime time.Time
	Err        error
}

func (ConfigsReloadedAction) Action() {}

type DockerComposeEventAction struct {
	Event dockercompose.Event
}

func (DockerComposeEventAction) Action() {}

type DockerComposeLogAction struct {
	ManifestName model.ManifestName
	Log          []byte
}

func (DockerComposeLogAction) Action() {}
