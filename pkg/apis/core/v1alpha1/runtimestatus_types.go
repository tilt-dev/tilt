package v1alpha1

// The RuntimeStatus is a simple, high-level summary of the runtime state of a server.
type RuntimeStatus string

const (
	// The server runtime status hasn't been read yet.
	RuntimeStatusUnknown RuntimeStatus = "unknown"

	// The server runtime is OK and passing health checks.
	RuntimeStatusOK RuntimeStatus = "ok"

	// The server runtime is still being scheduled or waiting on health checks.
	RuntimeStatusPending RuntimeStatus = "pending"

	// The server runtime is in an error state.
	RuntimeStatusError RuntimeStatus = "error"

	// There's no server runtime for this resource and never will be.
	RuntimeStatusNotApplicable RuntimeStatus = "not_applicable"
)
