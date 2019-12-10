package configs

import (
	"fmt"

	"github.com/windmilleng/tilt/internal/store"
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

func (w *tiltfileLogWriter) Write(p []byte) (n int, err error) {
	w.store.Dispatch(TiltfileLogAction{
		LogEvent: store.NewLogEvent(model.TiltfileManifestName, SpanIDForLoadCount(w.loadCount), p),
	})
	return len(p), nil
}

func SpanIDForLoadCount(loadCount int) logstore.SpanID {
	return logstore.SpanID(fmt.Sprintf("tilfile:%d", loadCount))
}
