package logging

import (
	"context"
	"log"
	"os"

	"path/filepath"

	"github.com/grpc-ecosystem/go-grpc-middleware/logging/logrus/ctxlogrus"
	hctxlogrus "github.com/improbable-eng/go-httpwares/logging/logrus/ctxlogrus"
	"github.com/sirupsen/logrus"
	"github.com/windmilleng/windmill/server/wmservice"
)

var globalLogEntry *logrus.Entry

func init() {
	log.SetFlags(log.Ltime | log.Lmicroseconds)
	logrus.SetFormatter(&logrus.JSONFormatter{})
	globalLogEntry = logrus.WithFields(logrus.Fields{})
}

// Grab an entry from the global logger.
// If you have a context, prefer logging.With(ctx)
func Global() *logrus.Entry {
	return globalLogEntry
}

func With(ctx context.Context) *logrus.Entry {
	entry := ctxlogrus.Extract(ctx)
	if len(entry.Data) != 0 {
		return entry
	}

	hEntry := hctxlogrus.Extract(ctx)
	if len(hEntry.Data) != 0 {
		return hEntry
	}

	// If we can't find a reasonable entry on the context,
	// fallback to the global entry.
	return Global()
}

func AddFields(ctx context.Context, fields logrus.Fields) {
	ctxlogrus.AddFields(ctx, fields)
	hctxlogrus.AddFields(ctx, fields)
}

// Sets up the logger to prints to [dir]/[name].log if dir is specified.
func SetupLogger(dir string) error {
	serviceName, err := wmservice.ServiceName()
	if err != nil {
		return err
	}

	globalLogEntry = logrus.WithFields(logrus.Fields{
		"service": serviceName,
	})

	if dir == "" {
		return nil
	}

	err = os.MkdirAll(dir, 0755)
	if err != nil {
		return err
	}

	logfile := filepath.Join(dir, "main.log")
	f, err := os.OpenFile(logfile, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0755)
	if err != nil {
		return err
	}

	Global().Infof("Redirecting logs to %s", logfile)
	logrus.SetOutput(f)
	log.SetOutput(f)
	Global().Infof("Logging initialized for %s", serviceName)

	return nil
}
