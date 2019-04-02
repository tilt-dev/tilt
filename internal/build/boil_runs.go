package build

import (
	"github.com/windmilleng/tilt/internal/model"
)

func BoilRuns(runs []model.Run, pathMappings []PathMapping) ([]model.Cmd, error) {
	res := []model.Cmd{}
	localPaths := PathMappingsToLocalPaths(pathMappings)
	for _, run := range runs {
		if run.Triggers.Empty() {
			res = append(res, run.Cmd)
			continue
		}

		anyMatch, err := model.AnyMatchGlobs(localPaths, run.Triggers)
		if err != nil {
			return nil, err
		}

		if anyMatch {
			res = append(res, run.Cmd)
		}
	}
	return res, nil
}
