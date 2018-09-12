package build

import (
	"github.com/windmilleng/tilt/internal/model"
)

func BoilSteps(steps []model.Step, pathMappings []pathMapping) ([]model.Cmd, error) {
	res := []model.Cmd{}
	for _, step := range steps {
		if step.Trigger == nil {
			res = append(res, step.Cmd)
			continue
		}
		for _, pm := range pathMappings {
			matches, err := step.Trigger.Matches(pm.LocalPath, false)
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
