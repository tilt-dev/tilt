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

func (w *tiltfileLogWriter) Write(level logger.Level, fields logger.Fields, p []byte) error {
	w.store.Dispatch(store.NewLogAction(model.TiltfileManifestName, SpanIDForLoadCount(w.loadCount), level, fields, p))
	return nil
}

func SpanIDForLoadCount(loadCount int) logstore.SpanID {
	return logstore.SpanID(fmt.Sprintf("tilfile:%d", loadCount))
}
