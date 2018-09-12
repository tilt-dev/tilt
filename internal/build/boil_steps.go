package build

import "github.com/windmilleng/tilt/internal/model"

type boiledStep struct {
	cmd         model.Cmd
	pathMapping pathMapping
}

func boilSteps(steps []model.Step, filesChanged []string, pathMappings []pathMapping) ([]boiledStep, error) {
	if len(filesChanged) == 0 {
		return []boiledStep{}, nil
	}
	res := []boiledStep{}
	for _, step := range steps {
		if step.Trigger == nil {
			res = append(res, boiledStep{cmd: step.Cmd})
			continue
		}
		for _, f := range filesChanged {
			matches, err := step.Trigger.Matches(f, false)
			if err != nil {
				return []boiledStep{}, err
			}
			if matches {
				res = append(res, boiledStep{cmd: step.Cmd})
			}
		}
	}
	return res, nil
}
