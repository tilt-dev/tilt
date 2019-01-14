package build

import (
	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/model"
)

func BoilSteps(steps []model.Step, pathMappings []PathMapping) ([]model.Cmd, error) {
	res := []model.Cmd{}
	for _, step := range steps {
		if step.Triggers == nil {
			res = append(res, step.Cmd)
			continue
		}
		matcher, err := ignore.CreateStepMatcher(step)
		if err != nil {
			return []model.Cmd{}, err
		}
		for _, pm := range pathMappings {
			matches, err := matcher.Matches(pm.LocalPath, false)
			if err != nil {
				return []model.Cmd{}, err
			}
			if matches {
				res = append(res, step.Cmd)
				break
			}
		}
	}
	return res, nil
}
