package apiserver

import "github.com/tilt-dev/tilt/pkg/model"

type SyncAction struct {
	Manifest model.Manifest
}

func (SyncAction) Action() {}
