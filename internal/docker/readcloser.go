package docker

import "io"

type readCloserWrapper struct {
	wrapped  io.ReadCloser
	tearDown func() error
}

var _ io.ReadCloser = readCloserWrapper{}

func WrapReadCloserWithTearDown(wrapped io.ReadCloser, tearDown func() error) readCloserWrapper {
	return readCloserWrapper{
		wrapped:  wrapped,
		tearDown: tearDown,
	}
}

func (w readCloserWrapper) Read(b []byte) (int, error) {
	return w.wrapped.Read(b)
}

func (w readCloserWrapper) Close() error {
	err1 := w.wrapped.Close()
	err2 := w.tearDown()
	if err1 != nil {
		return err1
	}
	return err2
}
