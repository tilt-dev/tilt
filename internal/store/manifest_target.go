package store

import (
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
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

func (mt *ManifestTarget) UpdateStatus() v1alpha1.UpdateStatus {
	m := mt.Manifest
	us := mt.State.UpdateStatus(m.TriggerMode)

	if us == v1alpha1.UpdateStatusPending {
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
		if us == v1alpha1.UpdateStatusInProgress {
			return v1alpha1.UpdateStatusPending
		}

		// If the local_resource has auto_init=False and has not built yet (i.e. it has
		// not been manually triggered), use UpdateStatusNone to indicate that the resource
		// has not yet had a reason to trigger so that it blocks the serve_cmd from executing.
		// Once manually triggered, a no-op build will exist, and subsequent calls will return
		// UpdateStatusNotApplicable so that the server controller knows it does not need to
		// wait for anything.
		if !m.TriggerMode.AutoInitial() && !mt.State.StartedFirstBuild() {
			return v1alpha1.UpdateStatusNone
		}
		return v1alpha1.UpdateStatusNotApplicable
	}

	return us
}

var _ model.Target = &ManifestTarget{}
