package model

import "time"

// Prune Docker objects older than this
const DockerPruneDefaultMaxAge = time.Hour * 6

// How often to prune Docker images while Tilt is running
const DockerPruneDefaultInterval = time.Hour

type DockerPruneSettings struct {
	Enabled   bool
	MaxAge    time.Duration // "prune Docker objects older than X"
	NumBuilds int           // "prune every Y builds" (takes precedence over "prune every Z hours")
	Interval  time.Duration // "prune every Z hours"
}

func DefaultDockerPruneSettings() DockerPruneSettings {
	// In code, disabled by default. (Note that in the Tiltfile, the default is
	// that Docker Prune is ENABLED -- so in `tilt up`, if user doesn't call
	// docker_prune_settings, pruning will be on by default. In `tilt down` etc.,
	// pruning will always be off.
	return DockerPruneSettings{Enabled: false}
}
