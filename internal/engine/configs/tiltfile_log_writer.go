package configs

import (
	"fmt"

	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
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
