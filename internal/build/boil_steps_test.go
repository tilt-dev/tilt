package build

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/model"
)

func TestBoilStepsNoTrigger(t *testing.T) {
	steps := []model.Step{
		model.Step{
			Cmd: model.ToShellCmd("echo hello"),
		},
	}

	fc := []string{"/home/tilt/code/test/foo"}
	pathMappings := []pathMapping{
		pathMapping{
			LocalPath:     "/home/tilt/code/test",
			ContainerPath: "/src",
		},
	}

	expected := []model.Cmd{model.ToShellCmd("echo hello")}

	actual, err := boilSteps(steps, fc, pathMappings)
	if err != nil {
		t.Fatal(err)
	}

	assert.ElementsMatch(t, expected, actual)
}

func TestBoilStepsNoFileschanged(t *testing.T) {
	steps := []model.Step{
		model.Step{
			Cmd: model.ToShellCmd("echo hello"),
		},
	}

	fc := []string{}
	pathMappings := []pathMapping{
		pathMapping{
			LocalPath:     "/home/tilt/code/test",
			ContainerPath: "/src",
		},
	}

	expected := []model.Cmd{}

	actual, err := boilSteps(steps, fc, pathMappings)
	if err != nil {
		t.Fatal(err)
	}

	assert.ElementsMatch(t, expected, actual)
}
