package runtimelog

import (
	"github.com/windmilleng/tilt/internal/k8s"
	"github.com/windmilleng/tilt/internal/store"
)

type PodLogAction struct {
	store.LogEvent
	PodID k8s.PodID
}

func (PodLogAction) Action() {}

type DockerComposeLogAction struct {
	store.LogEvent
}

func (DockerComposeLogAction) Action() {}
