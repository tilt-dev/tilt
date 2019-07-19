package errors

import (
	"context"
	"fmt"

	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/logger"
)

// Nothing is on fire, this is an expected case like a container builder being
// passed a build with no attached container.
// `Level` indicates at what log level this error should be shown to the user
type RedirectToNextBuilder struct {
	error
	Level logger.Level
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

func (r RedirectToNextBuilder) IsSilent() bool {
	return r.Level != logger.InfoLvl
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

var _ error = DontFallBackError{}

// A permanent error indicates that the whole build pipeline needs to stop.
// It will never recover, even on subsequent rebuilds.
func IsPermanentError(err error) bool {
	cause := errors.Cause(err)
	return cause == context.Canceled
}

func ShouldFallBackForErr(err error) bool {
	if IsPermanentError(err) {
		return false
	}

	cause := errors.Cause(err)
	if _, ok := cause.(DontFallBackError); ok {
		return false
	}
	return true
}
