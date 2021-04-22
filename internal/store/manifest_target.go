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
		// NOTE(nick): We currently model a local_resource(serve_cmd) as a Manifest
		// with a no-op Update. BuildController treats it like any other
		// resource. When the build completes, the server controller starts the
		// server.
		//
		// We want to make sure that the UpdateStatus is still "Pending" until we
		// have a completed build record. Otherwise the server controller will try
		// to start the server twice (once while the update is in-progress, and once
		// when the update completes).
		if us == model.UpdateStatusInProgress {
			return model.UpdateStatusPending
		}
		return model.UpdateStatusNotApplicable
	}

	return us
}

var _ model.Target = &ManifestTarget{}
