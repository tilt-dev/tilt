package engine

import (
	"net/url"
	"time"

	v1 "k8s.io/api/core/v1"

	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/store"
)

func NewErrorAction(err error) store.ErrorAction {
	return store.NewErrorAction(err)
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
	logEvent
	ManifestName model.ManifestName
}

func (BuildLogAction) Action() {}

type PodLogAction struct {
	logEvent
	ManifestName model.ManifestName
	PodID        k8s.PodID
}

func (PodLogAction) Action() {}

type LogAction struct {
	logEvent
}

func (LogAction) Action() {}

type DeployIDAction struct {
	TargetID model.TargetID
	DeployID model.DeployID
}

func (DeployIDAction) Action() {}

func NewDeployIDAction(id model.TargetID, dID model.DeployID) DeployIDAction {
	return DeployIDAction{
		TargetID: id,
		DeployID: dID,
	}

}
func NewDeployIDActionsForTargets(ids []model.TargetID, dID model.DeployID) []DeployIDAction {
	actions := make([]DeployIDAction, len(ids))
	for i, id := range ids {
		actions[i] = DeployIDAction{
			TargetID: id,
			DeployID: dID,
		}
	}
	return actions
}

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
	GlobalYAMLManifest model.Manifest
	TiltfilePath       string
	ConfigFiles        []string
	InitManifests      []model.ManifestName
	TriggerMode        model.TriggerMode

	StartTime  time.Time
	FinishTime time.Time
	Err        error
	Warnings   []string

	ExecuteTiltfile bool
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
	StartTime    time.Time
}

func (ConfigsReloadStartedAction) Action() {}

type ConfigsReloadedAction struct {
	Manifests          []model.Manifest
	GlobalYAML         model.Manifest
	TiltIgnoreContents string
	ConfigFiles        []string

	StartTime  time.Time
	FinishTime time.Time
	Err        error
	Warnings   []string
}

func (ConfigsReloadedAction) Action() {}

type DockerComposeEventAction struct {
	Event dockercompose.Event
}

func (DockerComposeEventAction) Action() {}

type DockerComposeLogAction struct {
	logEvent
	ManifestName model.ManifestName
}

func (DockerComposeLogAction) Action() {}

type TiltfileLogAction struct {
	logEvent
}

func (TiltfileLogAction) Action() {}

type logEvent struct {
	ts      time.Time
	message []byte
}

func (le logEvent) Time() time.Time {
	return le.ts
}

func (le logEvent) Message() []byte {
	return le.message
}

func newLogEvent(b []byte) logEvent {
	return logEvent{
		ts:      time.Now(),
		message: b,
	}
}
