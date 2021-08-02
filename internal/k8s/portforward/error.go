package portforward

import "sync"

// A little data structure to get the first error
// from many connections.
type errorHandler struct {
	once  sync.Once
	errCh chan error
}

func newErrorHandler() *errorHandler {
	return &errorHandler{
		errCh: make(chan error),
	}
}

func (h *errorHandler) Close() {
	h.once.Do(func() {})
	close(h.errCh)
}

func (h *errorHandler) Stop(err error) {
	h.once.Do(func() {
		h.errCh <- err
	})
}

func (h *errorHandler) Done() chan error {
	return h.errCh
}
