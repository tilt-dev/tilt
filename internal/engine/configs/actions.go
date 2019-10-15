package configs

import (
	"time"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/model"
)

type ConfigsReloadStartedAction struct {
	FilesChanged map[string]bool
	StartTime    time.Time
}

func (ConfigsReloadStartedAction) Action() {}

type ConfigsReloadedAction struct {
	Manifests          []model.Manifest
	TiltIgnoreContents string
	ConfigFiles        []string

	FinishTime          time.Time
	Err                 error
	Warnings            []string
	Features            map[string]bool
	TeamName            string
	Secrets             model.SecretSet
	DockerPruneSettings model.DockerPruneSettings

	// The length of the global log when Tiltfile execution started.
	// Useful for knowing how far back in time we have to scrub secrets.
	GlobalLogLineCountAtExecStart int
}

func (ConfigsReloadedAction) Action() {}

type TiltfileLogAction struct {
	store.LogEvent
}

func (TiltfileLogAction) Action() {}
