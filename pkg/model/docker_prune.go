package model

import "time"

type DockerPruneSettings struct {
	Disabled  bool
	Interval  time.Duration
	MaxAge    time.Duration
	NumBuilds int
}
