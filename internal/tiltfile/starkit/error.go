package starkit

import (
	"github.com/pkg/errors"
	"go.starlark.net/starlark"
)

// Keep unwrapping errors until we find an error with a backtrace.
func UnpackBacktrace(err error) error {
	var bestEvalError *starlark.EvalError
	current := err
	for {
		evalErr, ok := current.(*starlark.EvalError)
		if ok {
			bestEvalError = evalErr
		}

		wrapper, ok := current.(wrapper)
		if !ok {
			break
		}

		current = wrapper.Unwrap()
	}

	if bestEvalError != nil {
		return errors.New(bestEvalError.Backtrace())
	}
	return err
}

// go 1.13 error wrapper
type wrapper interface {
	Unwrap() error
}
