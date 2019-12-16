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

type InitAction struct {
	WatchFiles   bool
	TiltfilePath string
	ConfigFiles  []string
	UserArgs     []string

	TiltBuild model.TiltBuild
	StartTime time.Time

	AnalyticsUserOpt analytics.Opt

	CloudAddress string
	Token        token.Token
	HUDEnabled   bool
}

func (InitAction) Action() {}

type ManifestReloadedAction struct {
	OldManifest model.Manifest
	NewManifest model.Manifest
	Error       error
}

func (ManifestReloadedAction) Action() {}

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

type TelemetryScriptRanAction struct {
	At time.Time
}

func (TelemetryScriptRanAction) Action() {}
