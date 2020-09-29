package build

import (
	"github.com/tilt-dev/tilt/pkg/model"
)

func BoilRuns(runs []model.Run, pathMappings []PathMapping) ([]model.Cmd, error) {
	res := []model.Cmd{}
	localPaths := PathMappingsToLocalPaths(pathMappings)
	for _, run := range runs {
		if run.Triggers.Empty() {
			res = append(res, run.Cmd)
			continue
		}

		anyMatch, _, err := run.Triggers.AnyMatch(localPaths)
		if err != nil {
			return nil, err
		}

		if anyMatch {
			res = append(res, run.Cmd)
		}
	}
	return res, nil
}
