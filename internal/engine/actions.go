package engine

import (
	"net/url"
	"time"

	"github.com/windmilleng/wmclient/pkg/analytics"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"
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
	store.LogEvent
}

func (BuildLogAction) Action() {}

type PodLogAction struct {
	store.LogEvent
	PodID k8s.PodID
}

func (PodLogAction) Action() {}

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
	WatchFiles    bool
	TiltfilePath  string
	ConfigFiles   []string
	InitManifests []model.ManifestName

	TiltBuild model.TiltBuild
	StartTime time.Time

	EnableSail bool

	AnalyticsOpt analytics.Opt
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
	TiltIgnoreContents string
	ConfigFiles        []string

	FinishTime time.Time
	Err        error
	Warnings   []string
	Features   map[string]bool
	TeamName   string
}

func (ConfigsReloadedAction) Action() {}

type DockerComposeEventAction struct {
	Event dockercompose.Event
}

func (DockerComposeEventAction) Action() {}

type DockerComposeLogAction struct {
	store.LogEvent
}

func (DockerComposeLogAction) Action() {}

type TiltfileLogAction struct {
	store.LogEvent
}

func (TiltfileLogAction) Action() {}

type LatestVersionAction struct {
	Build model.TiltBuild
}

func (LatestVersionAction) Action() {}

type UIDUpdateAction struct {
	UID          k8s.UID
	EventType    watch.EventType
	ManifestName model.ManifestName
	Entity       k8s.K8sEntity
}

func (UIDUpdateAction) Action() {}
