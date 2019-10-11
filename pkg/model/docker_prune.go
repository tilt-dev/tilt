package model

import "time"

// How often to prune Docker images while Tilt is running
const dockerPruneDefaultInterval = time.Hour

// Prune Docker objects older than this
const dockerPruneDefaultMaxAge = time.Hour * 6

type DockerPruneSettings struct {
	Disabled  bool
	Interval  time.Duration
	MaxAge    time.Duration
	NumBuilds int
}

func DefaultDockerPruneSettings() DockerPruneSettings {
	return DockerPruneSettings{
		Interval: dockerPruneDefaultInterval,
		MaxAge:   dockerPruneDefaultMaxAge,
	}
}
