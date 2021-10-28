package buildcontrols

import (
	"context"

	"github.com/pkg/errors"
)

// A permanent error indicates that the whole build pipeline needs to stop.
// It will never recover, even on subsequent rebuilds.
func IsFatalError(err error) bool {
	cause := errors.Cause(err)
	return cause == context.Canceled
}
