package engine

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/logger"
)

// Nothing is on fire, this is an expected case like a container builder being
// passed a build with no attached container.
// `level` indicates at what log level this error should be shown to the user
type RedirectToNextBuilder struct {
	error
	level logger.Level
}

func WrapRedirectToNextBuilder(err error, level logger.Level) RedirectToNextBuilder {
	return RedirectToNextBuilder{err, level}
}

func SilentRedirectToNextBuilderf(msg string, a ...interface{}) RedirectToNextBuilder {
	// Only show to user in Debug mode
	return RedirectToNextBuilder{fmt.Errorf(msg, a...), logger.DebugLvl}
}

func RedirectToNextBuilderInfof(msg string, a ...interface{}) RedirectToNextBuilder {
	return RedirectToNextBuilder{fmt.Errorf(msg, a...), logger.InfoLvl}
}

var _ error = RedirectToNextBuilder{}

// Something is wrong enough that we shouldn't bother falling back to other
// BaD's -- they won't work.
type DontFallBackError struct {
	error
}

func WrapDontFallBackError(err error) DontFallBackError {
	return DontFallBackError{err}
}

func DontFallBackErrorf(msg string, a ...interface{}) DontFallBackError {
	return DontFallBackError{fmt.Errorf(msg, a...)}
}

func IsDontFallBackError(err error) bool {
	_, ok := err.(DontFallBackError)
	return ok
}

var _ error = DontFallBackError{}

// A permanent error indicates that the whole build pipeline needs to stop.
// It will never recover, even on subsequent rebuilds.
func isPermanentError(err error) bool {
	cause := errors.Cause(err)
	return cause == context.Canceled
}

func shouldFallBackForErr(err error) bool {
	if isPermanentError(err) {
		return false
	}

	cause := errors.Cause(err)
	if IsDontFallBackError(cause) {
		return false
	}
	return true
}
