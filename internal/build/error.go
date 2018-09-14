package build

import "fmt"

// Indicates that the build failed because the user script failed
// (as opposed to an infrastructure issue).
type UserBuildFailure struct {
	ExitCode int
}

func (e UserBuildFailure) Error() string {
	return fmt.Sprintf("Command failed with exit code: %d", e.ExitCode)
}

func IsUserBuildFailure(err error) bool {
	_, ok := err.(UserBuildFailure)
	return ok
}

var _ error = UserBuildFailure{}
