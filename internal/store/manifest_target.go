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
		State:    NewManifestState(m),
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

	if m.IsLocal() && m.LocalTarget().UpdateCmdSpec == nil {
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

		if us == v1alpha1.UpdateStatusOK {
			// The no-op build job completed, but it's confusing/misleading to show
			// the status of something non-existent as having succeeded, so instead
			// return the special N/A status so that it can be distinguished from a
			// true update.
			//
			// Note that for local resources with auto_init=False that have not been
			// triggered, the update status will be UpdateStatusNone until such a time
			// as they are triggered, and will be UpdateStatusNotApplicable thereafter.
			//
			// This is a bit odd, but currently this is how the server controller
			// determines whether to launch the serve_cmd, so it needs to be able to
			// distinguish between "resource has never been triggered" (so the server
			// should not be launched) and "resource has been triggered but has no
			// update command to wait for" (and thus the server should be launched).
			return v1alpha1.UpdateStatusNotApplicable
		}
	}

	return us
}

// Compute the runtime status for the whole Manifest.
func (mt *ManifestTarget) RuntimeStatus() v1alpha1.RuntimeStatus {
	m := mt.Manifest
	if m.IsLocal() && m.LocalTarget().ServeCmd.Empty() {
		return v1alpha1.RuntimeStatusNotApplicable
	}
	return mt.State.RuntimeStatus(m.TriggerMode)
}

var _ model.Target = &ManifestTarget{}
