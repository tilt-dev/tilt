package build

import (
	"errors"
)

// RunStepFailure indicates that the update failed because one of the user's
// Runs failed (i.e. exited non-zero) -- as opposed to an infrastructure issue.
type RunStepFailure struct {
	err error
}

func NewRunStepFailure(err error) RunStepFailure {
	return RunStepFailure{err: err}
}

func (e RunStepFailure) Unwrap() error {
	return e.err
}

func (e RunStepFailure) Error() string {
	if e.err != nil {
		return e.err.Error()
	}
	return ""
}

func IsRunStepFailure(err error) bool {
	var rsf RunStepFailure
	if errors.As(err, &rsf) {
		return true
	}
	return false
}

var _ error = RunStepFailure{}
