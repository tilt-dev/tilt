// Go's error library is very minimalist.
//
// Go doesn't express an opinionated way to do common error operations, like
// error codes and error causes (in contrast to Java's more full-featured
// Throwable hierarchy).
//
// Sadly, this means that every third-party Go library implements their
// own way of doing these things (e.g., context.Canceled vs grpc/codes.Canceled).
//
// The purpose of this package is to normalize error-handling across these
// third-party libraries.

package errors

import (
	"context"
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func IsCanceled(err error) bool {
	if err == context.Canceled {
		return true
	}
	code := grpc.Code(err)
	if code == codes.Canceled {
		return true
	}
	return false
}

func IsDeadlineExceeded(err error) bool {
	if err == context.DeadlineExceeded {
		return true
	}
	code := grpc.Code(err)
	if code == codes.DeadlineExceeded {
		return true
	}
	return false
}

// Propagatef follows the convention that any function ending in 'f' takes
// a format string. We also tell go vet about this function so that it can
// check the format string validity.
func Propagatef(err error, msg string, args ...interface{}) error {
	st, ok := status.FromError(err)
	if ok {
		return grpc.Errorf(st.Code(), "%s: %v", fmt.Errorf(msg, args...), st.Message())
	}
	if IsDeadlineExceeded(err) {
		return grpc.Errorf(codes.DeadlineExceeded, "%s: %v", fmt.Errorf(msg, args...), err)
	}
	if IsCanceled(err) {
		return grpc.Errorf(codes.Canceled, "%s: %v", fmt.Errorf(msg, args...), err)
	}
	return fmt.Errorf("%s: %v", fmt.Errorf(msg, args...), err)
}
