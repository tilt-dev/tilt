package configs

import (
	"fmt"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

type tiltfileLogWriter struct {
	name      model.ManifestName
	store     store.RStore
	loadCount int
}

func NewTiltfileLogWriter(mn model.ManifestName, s store.RStore, loadCount int) *tiltfileLogWriter {
	return &tiltfileLogWriter{mn, s, loadCount}
}

func (w *tiltfileLogWriter) Write(level logger.Level, fields logger.Fields, p []byte) error {
	w.store.Dispatch(store.NewLogAction(w.name, SpanIDForLoadCount(w.loadCount), level, fields, p))
	return nil
}

func SpanIDForLoadCount(loadCount int) logstore.SpanID {
	return logstore.SpanID(fmt.Sprintf("tiltfile:%d", loadCount))
}
