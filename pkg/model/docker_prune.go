package model

import "time"

type DockerPruneSettings struct {
	Enabled   bool
	MaxAge    time.Duration // "prune Docker objects older than X"
	NumBuilds int           // "prune every Y builds" (takes precedence over "prune every Z hours")
	Interval  time.Duration // "prune every Z hours"
}
