package store

import (
	"github.com/tilt-dev/tilt/pkg/model"
)

type ManifestTarget struct {
	Manifest model.Manifest
	State    *ManifestState
}

func NewManifestTarget(m model.Manifest) *ManifestTarget {
	return &ManifestTarget{
		Manifest: m,
		State:    newManifestState(m),
	}
}

func (t ManifestTarget) Spec() model.TargetSpec {
	return t.Manifest
}

func (t ManifestTarget) Status() model.TargetStatus {
	return t.State
}

func (mt *ManifestTarget) UpdateStatus() model.UpdateStatus {
	m := mt.Manifest
	us := mt.State.UpdateStatus(m.TriggerMode)

	if us == model.UpdateStatusPending {
		// A resource with no update command can still be in pending mode.
		return us
	}

	if m.IsLocal() && m.LocalTarget().UpdateCmd.Empty() {
		return model.UpdateStatusNotApplicable
	}

	return us
}

var _ model.Target = &ManifestTarget{}
