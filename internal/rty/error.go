package rty

// We want our tests to be able to handle errors.
//
// But we don't want errors in RTY rendering to stop the rendering pipeline.
//
// So we need a way to accumulate errors.
// By design, testing.T implements this interface.
type ErrorHandler interface {
	Errorf(format string, args ...interface{})
}

type SkipErrorHandler struct{}

func (SkipErrorHandler) Errorf(format string, args ...interface{}) {
	// do nothing
}
