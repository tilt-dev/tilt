package http_logrus

import (
	"log"

	"github.com/sirupsen/logrus"
)

// AsHttpLogger returns the given logrus instance as an HTTP logger.
func AsHttpLogger(logger *logrus.Entry) *log.Logger {
	return log.New(&loggerWriter{logger: logger.WithField("system", SystemField)}, "", 0)
}

// loggerWriter is needed to use a Writer so that you can get a std log.Logger.
type loggerWriter struct {
	logger *logrus.Entry
}

func (w *loggerWriter) Write(p []byte) (n int, err error) {
	w.logger.Warn(string(p))
	return len(p), nil
}
