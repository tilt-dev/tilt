package tiltfile

import (
	"fmt"

	"github.com/tilt-dev/tilt/pkg/model"
	"github.com/tilt-dev/tilt/pkg/model/logstore"
)

func SpanIDForLoadCount(mn model.ManifestName, loadCount int) logstore.SpanID {
	return logstore.SpanID(fmt.Sprintf("tiltfile:%s:%d", mn, loadCount))
}
