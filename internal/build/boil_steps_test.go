package build

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/dockerignore"
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

	expected := []boiledStep{boiledStep{cmd: model.ToShellCmd("echo hello")}}

	actual, err := boilSteps(steps, fc, pathMappings)
	if err != nil {
		t.Fatal(err)
	}

	assert.ElementsMatch(t, expected, actual)
}

func TestBoilStepsNoFilesChanged(t *testing.T) {
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

	expected := []boiledStep{}

	actual, err := boilSteps(steps, fc, pathMappings)
	if err != nil {
		t.Fatal(err)
	}

	assert.ElementsMatch(t, expected, actual)
}

func TestBoilStepsOneTriggerFilesDontMatch(t *testing.T) {
	trigger, err := dockerignore.NewDockerPatternMatcher("/home/tilt/code/test", []string{"bar"})
	if err != nil {
		t.Fatal(err)
	}
	steps := []model.Step{
		model.Step{
			Cmd:     model.ToShellCmd("echo hello"),
			Trigger: trigger,
		},
	}

	fc := []string{"/home/tilt/code/test/foo"}
	pathMappings := []pathMapping{
		pathMapping{
			LocalPath:     "/home/tilt/code/test",
			ContainerPath: "/src",
		},
	}

	expected := []boiledStep{}

	actual, err := boilSteps(steps, fc, pathMappings)
	if err != nil {
		t.Fatal(err)
	}

	assert.ElementsMatch(t, expected, actual)
}

func TestBoilStepsOneTriggerMatchigFile(t *testing.T) {
	trigger, err := dockerignore.NewDockerPatternMatcher("/home/tilt/code/test", []string{"bar"})
	if err != nil {
		t.Fatal(err)
	}
	steps := []model.Step{
		model.Step{
			Cmd:     model.ToShellCmd("echo world"),
			Trigger: trigger,
		},
	}

	fc := []string{"/home/tilt/code/test/bar"}
	pathMappings := []pathMapping{
		pathMapping{
			LocalPath:     "/home/tilt/code/test",
			ContainerPath: "/src",
		},
	}

	expected := []boiledStep{boiledStep{cmd: model.ToShellCmd("echo world")}}

	actual, err := boilSteps(steps, fc, pathMappings)
	if err != nil {
		t.Fatal(err)
	}

	assert.ElementsMatch(t, expected, actual)
}
