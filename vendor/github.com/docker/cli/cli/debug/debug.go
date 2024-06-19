package debug

import (
	"os"

	"github.com/sirupsen/logrus"
	"go.opentelemetry.io/otel"
)

// Enable sets the DEBUG env var to true
// and makes the logger to log at debug level.
func Enable() {
	os.Setenv("DEBUG", "1")
	logrus.SetLevel(logrus.DebugLevel)
}

// Disable sets the DEBUG env var to false
// and makes the logger to log at info level.
func Disable() {
	os.Setenv("DEBUG", "")
	logrus.SetLevel(logrus.InfoLevel)
}

// IsEnabled checks whether the debug flag is set or not.
func IsEnabled() bool {
	return os.Getenv("DEBUG") != ""
}

// OTELErrorHandler is an error handler for OTEL that
// uses the CLI debug package to log messages when an error
// occurs.
//
// The default is to log to the debug level which is only
// enabled when debugging is enabled.
var OTELErrorHandler otel.ErrorHandler = otel.ErrorHandlerFunc(func(err error) {
	logrus.WithError(err).Debug("otel error")
})
