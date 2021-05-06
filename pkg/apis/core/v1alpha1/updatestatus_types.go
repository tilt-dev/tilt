package v1alpha1

// The UpdateStatus is a simple, high-level summary of any update tasks to bring
// the resource up-to-date.
type UpdateStatus string

const (
	// This resource hasn't had any reason to update yet.
	// This usually indicates that it's a manual trigger with no auto_init.
	UpdateStatusNone UpdateStatus = "none"

	// An update is in progress.
	UpdateStatusInProgress UpdateStatus = "in_progress"

	// The last update succeeded.
	UpdateStatusOK UpdateStatus = "ok"

	// An update is queued.
	UpdateStatusPending UpdateStatus = "pending"

	// The last update failed.
	UpdateStatusError UpdateStatus = "error"

	// This resource doesn't have an update command.
	UpdateStatusNotApplicable UpdateStatus = "not_applicable"
)
