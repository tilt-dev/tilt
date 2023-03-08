package dockercomposelogstream

import (
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/internal/store/dockercomposeservices"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type LogActionWriter struct {
	store        store.RStore
	manifestName model.ManifestName
}

func (w *LogActionWriter) Write(p []byte) (n int, err error) {
	w.store.Dispatch(store.NewLogAction(w.manifestName,
		dockercomposeservices.SpanIDForDCService(w.manifestName), logger.InfoLvl, nil, p))
	return len(p), nil
}
