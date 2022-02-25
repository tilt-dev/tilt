package buildcontrols

import (
	"context"

	"github.com/tilt-dev/tilt/internal/ospath"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type BuildEntry struct {
	Name         model.ManifestName
	BuildReason  model.BuildReason
	FilesChanged []string
}

func LogBuildEntry(ctx context.Context, entry BuildEntry) {
	buildReason := entry.BuildReason
	changedFiles := entry.FilesChanged
	firstBuild := buildReason.Has(model.BuildReasonFlagInit)

	l := logger.Get(ctx).WithFields(logger.Fields{logger.FieldNameBuildEvent: "init"})
	if firstBuild {
		l.Infof("Initial Build")
	} else {
		if len(changedFiles) > 0 {
			t := "File"
			if len(changedFiles) > 1 {
				t = "Files"
			}
			l.Infof("%d %s Changed: %s", len(changedFiles), t, ospath.FormatFileChangeList(changedFiles))
		} else {
			l.Infof("%s", buildReason)
		}
	}
}
