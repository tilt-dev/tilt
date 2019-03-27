package build

import (
	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/model"
)

func BoilRuns(runs []model.Run, pathMappings []PathMapping) ([]model.Cmd, error) {
	res := []model.Cmd{}
	for _, run := range runs {
		if run.Triggers == nil {
			res = append(res, run.Cmd)
			continue
		}
		matcher, err := ignore.CreateRunMatcher(run)
		if err != nil {
			return []model.Cmd{}, err
		}
		for _, pm := range pathMappings {
			matches, err := matcher.Matches(pm.LocalPath, false)
			if err != nil {
				return []model.Cmd{}, err
			}
			if matches {
				res = append(res, run.Cmd)
				break
			}
		}
	}
	return res, nil
}
