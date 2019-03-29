package build

import (
	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/model"
)

func BoilRuns(runs []model.Run, pathMappings []PathMapping) ([]model.Cmd, error) {
	res := []model.Cmd{}
	localPaths := PathMappingsToLocalPaths(pathMappings)
	for _, run := range runs {
		if run.Triggers == nil {
			res = append(res, run.Cmd)
			continue
		}

		anyMatch, err := ignore.MatchesAnyPaths(run.Triggers, localPaths, run.BaseDirectory)
		if err != nil {
			return nil, err
		}

		if anyMatch {
			res = append(res, run.Cmd)
		}
	}
	return res, nil
}
