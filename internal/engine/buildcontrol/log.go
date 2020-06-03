package buildcontrol

import (
	"context"

	"github.com/tilt-dev/tilt/internal/ospath"
	"github.com/tilt-dev/tilt/pkg/logger"
	"github.com/tilt-dev/tilt/pkg/model"
)

type BuildEntry interface {
	Name() model.ManifestName
	BuildReason() model.BuildReason
	FilesChanged() []string
}

func LogBuildEntry(ctx context.Context, entry BuildEntry) {
	name := entry.Name()
	buildReason := entry.BuildReason()
	changedFiles := entry.FilesChanged()
	firstBuild := buildReason.Has(model.BuildReasonFlagInit)

	l := logger.Get(ctx).WithFields(logger.Fields{logger.FieldNameBuildEvent: "init"})
	delimiter := "•"
	if firstBuild {
		l.Infof("Initial Build %s %s", delimiter, name)
	} else {
		if len(changedFiles) > 0 {
			t := "File"
			if len(changedFiles) > 1 {
				t = "Files"
			}
			l.Infof("%d %s Changed: %s %s %s", len(changedFiles), t, ospath.FormatFileChangeList(changedFiles), delimiter, name)
		} else {
			l.Infof("%s %s %s", buildReason, delimiter, name)
		}
	}
}
