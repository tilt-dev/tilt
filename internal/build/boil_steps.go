package build

import "github.com/windmilleng/tilt/internal/model"

func boilSteps(steps []model.Step, filesChanged []string, pathMappings []pathMapping) ([]model.Cmd, error) {
	if len(filesChanged) == 0 {
		return []model.Cmd{}, nil
	}
	res := make([]model.Cmd, len(steps))
	for i, step := range steps {
		res[i] = step.Cmd
	}
	return res, nil
}
