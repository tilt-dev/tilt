package build

import "github.com/windmilleng/tilt/internal/model"

func boilSteps(steps []model.Step, filesChanged []string, pathMappings []pathMapping) ([]model.Cmd, error) {
	if len(filesChanged) == 0 {
		return []model.Cmd{}, nil
	}
	res := []model.Cmd{}
	for _, step := range steps {
		if step.Trigger == nil {
			res = append(res, step.Cmd)
			continue
		}
		for _, f := range filesChanged {
			matches, err := step.Trigger.Matches(f, false)
			if err != nil {
				return []model.Cmd{}, err
			}
			if matches {
				res = append(res, step.Cmd)
			}
		}
	}
	return res, nil
}
