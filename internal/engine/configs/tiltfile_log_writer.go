package configs

import (
	"fmt"

	"github.com/windmilleng/tilt/internal/store"
	"github.com/windmilleng/tilt/pkg/logger"
	"github.com/windmilleng/tilt/pkg/model"
	"github.com/windmilleng/tilt/pkg/model/logstore"
)

type tiltfileLogWriter struct {
	store     store.RStore
	loadCount int
}

func NewTiltfileLogWriter(s store.RStore, loadCount int) *tiltfileLogWriter {
	return &tiltfileLogWriter{s, loadCount}
}

func (w *tiltfileLogWriter) Write(level logger.Level, p []byte) error {
	w.store.Dispatch(TiltfileLogAction{
		LogEvent: store.NewLogEvent(model.TiltfileManifestName, SpanIDForLoadCount(w.loadCount), level, p),
	})
	return nil
}

func SpanIDForLoadCount(loadCount int) logstore.SpanID {
	return logstore.SpanID(fmt.Sprintf("tilfile:%d", loadCount))
}
