package engine

import (
	"time"

	"github.com/windmilleng/wmclient/pkg/analytics"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/windmilleng/tilt/internal/dockercompose"
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/internal/token"
	"github.com/windmilleng/tilt/pkg/model"
)

func NewErrorAction(err error) store.ErrorAction {
	return store.NewErrorAction(err)
}

type BuildLogAction struct {
	store.LogEvent
}

func (BuildLogAction) Action() {}

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

	AnalyticsOpt analytics.Opt

	CloudAddress string
	Token        token.Token
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

type DockerComposeEventAction struct {
	Event dockercompose.Event
}

func (DockerComposeEventAction) Action() {}

type LatestVersionAction struct {
	Build model.TiltBuild
}

func (LatestVersionAction) Action() {}

type UIDUpdateAction struct {
	UID          types.UID
	EventType    watch.EventType
	ManifestName model.ManifestName
	Entity       k8s.K8sEntity
}

func (UIDUpdateAction) Action() {}
