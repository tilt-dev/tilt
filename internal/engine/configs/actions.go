package configs

import (
	"time"

	"github.com/windmilleng/wmclient/pkg/analytics"

	"github.com/windmilleng/tilt/pkg/model"
	"github.com/windmilleng/tilt/pkg/model/logstore"
)

type ConfigsReloadStartedAction struct {
	FilesChanged map[string]bool
	StartTime    time.Time
	SpanID       logstore.SpanID
}

func (ConfigsReloadStartedAction) Action() {}

type ConfigsReloadedAction struct {
	Manifests          []model.Manifest
	TiltIgnoreContents string
	ConfigFiles        []string

	FinishTime           time.Time
	Err                  error
	Warnings             []string
	Features             map[string]bool
	TeamName             string
	TelemetrySettings    model.TelemetrySettings
	Secrets              model.SecretSet
	DockerPruneSettings  model.DockerPruneSettings
	AnalyticsTiltfileOpt analytics.Opt
	VersionSettings      model.VersionSettings
	UpdateSettings       model.UpdateSettings

	// A checkpoint into the logstore when Tiltfile execution started.
	// Useful for knowing how far back in time we have to scrub secrets.
	CheckpointAtExecStart logstore.Checkpoint
}

func (ConfigsReloadedAction) Action() {}
