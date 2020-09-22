package configs

import (
	"time"

	"github.com/tilt-dev/wmclient/pkg/analytics"

	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

type ConfigsReloadStartedAction struct {
	FilesChanged []string
	StartTime    time.Time
	SpanID       logstore.SpanID
	Reason       model.BuildReason
}

func (ConfigsReloadStartedAction) Action() {}

type ConfigsReloadedAction struct {
	// TODO(nick): Embed TiltfileLoadResult instead of copying fields.
	Manifests   []model.Manifest
	Tiltignore  model.Dockerignore
	ConfigFiles []string

	FinishTime           time.Time
	Err                  error
	Warnings             []string
	Features             map[string]bool
	TeamID               string
	TelemetrySettings    model.TelemetrySettings
	MetricsSettings      model.MetricsSettings
	Secrets              model.SecretSet
	DockerPruneSettings  model.DockerPruneSettings
	AnalyticsTiltfileOpt analytics.Opt
	VersionSettings      model.VersionSettings
	UpdateSettings       model.UpdateSettings
	WatchSettings        model.WatchSettings

	// A checkpoint into the logstore when Tiltfile execution started.
	// Useful for knowing how far back in time we have to scrub secrets.
	CheckpointAtExecStart logstore.Checkpoint
}

func (ConfigsReloadedAction) Action() {}
