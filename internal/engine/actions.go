package engine

import (
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"

	"github.com/tilt-dev/tilt/internal/k8s"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/token"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/wmclient/pkg/analytics"
)

func NewErrorAction(err error) store.ErrorAction {
	return store.NewErrorAction(err)
}

type InitAction struct {
	TiltfilePath string
	UserArgs     []string

	TiltBuild model.TiltBuild
	StartTime time.Time

	AnalyticsUserOpt analytics.Opt

	CloudAddress string
	Token        token.Token
	TerminalMode store.TerminalMode
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
