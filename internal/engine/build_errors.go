package engine

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
)

// Nothing is on fire, this is an expected case like a container builder being
// passed a build with no attached container. Don't need to show this to a user.
type RedirectToNextBuilder struct {
	error
}

func WrapRedirectToNextBuilder(err error) RedirectToNextBuilder {
	return RedirectToNextBuilder{err}
}

func RedirectToNextBuilderf(msg string, a ...interface{}) RedirectToNextBuilder {
	return RedirectToNextBuilder{fmt.Errorf(msg, a...)}
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
func isPermanentError(err error) bool {
	cause := errors.Cause(err)
	if cause == context.Canceled {
		return true
	}
	return false
}

func shouldFallBackForErr(err error) bool {
	if isPermanentError(err) {
		return false
	}

	cause := errors.Cause(err)
	if _, ok := cause.(DontFallBackError); ok {
		return false
	}
	return true
}
