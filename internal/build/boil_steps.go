package build

import (
	"github.com/windmilleng/tilt/internal/model"
)

type BoiledStep struct {
	cmd         model.Cmd
	pathMapping pathMapping
}

func BoilSteps(steps []model.Step, pathMappings []pathMapping) ([]BoiledStep, error) {
	res := []BoiledStep{}
	for _, step := range steps {
		if step.Trigger == nil {
			res = append(res, BoiledStep{cmd: step.Cmd})
			continue
		}
		for _, pm := range pathMappings {
			matches, err := step.Trigger.Matches(pm.LocalPath, false)
			if err != nil {
				return []BoiledStep{}, err
			}
			if matches {
				res = append(res, BoiledStep{cmd: step.Cmd, pathMapping: pm})
				break
			}
		}
	}
	return res, nil
}
