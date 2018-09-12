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

	pathMappings := []pathMapping{
		pathMapping{
			LocalPath:     "/home/tilt/code/test/foo",
			ContainerPath: "/src/foo",
		},
	}

	expected := []BoiledStep{BoiledStep{cmd: model.ToShellCmd("echo hello")}}

	actual, err := BoilSteps(steps, pathMappings)
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

	pathMappings := []pathMapping{}

	expected := []BoiledStep{}

	actual, err := BoilSteps(steps, pathMappings)
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

	pathMappings := []pathMapping{
		pathMapping{
			LocalPath:     "/home/tilt/code/test/foo",
			ContainerPath: "/src/foo",
		},
	}

	expected := []BoiledStep{}

	actual, err := BoilSteps(steps, pathMappings)
	if err != nil {
		t.Fatal(err)
	}

	assert.ElementsMatch(t, expected, actual)
}

func TestBoilStepsOneTriggerMatchingFile(t *testing.T) {
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

	pathMappings := []pathMapping{
		pathMapping{
			LocalPath:     "/home/tilt/code/test/bar",
			ContainerPath: "/src/bar",
		},
	}

	expected := []BoiledStep{BoiledStep{cmd: model.ToShellCmd("echo world"), pathMapping: pathMappings[0]}}

	actual, err := BoilSteps(steps, pathMappings)
	if err != nil {
		t.Fatal(err)
	}

	assert.ElementsMatch(t, expected, actual)
}

func TestBoilStepsManyTriggersManyFiles(t *testing.T) {
	trigger1, err := dockerignore.NewDockerPatternMatcher("/home/tilt/code/test", []string{"foo"})
	if err != nil {
		t.Fatal(err)
	}
	trigger2, err := dockerignore.NewDockerPatternMatcher("/home/tilt/code/test", []string{"bar"})
	if err != nil {
		t.Fatal(err)
	}
	steps := []model.Step{
		model.Step{
			Cmd:     model.ToShellCmd("echo hello"),
			Trigger: trigger1,
		},
		model.Step{
			Cmd:     model.ToShellCmd("echo world"),
			Trigger: trigger2,
		},
	}

	pathMappings := []pathMapping{
		pathMapping{
			LocalPath:     "/home/tilt/code/test/baz",
			ContainerPath: "/src/baz",
		},
		pathMapping{
			LocalPath:     "/home/tilt/code/test/bar",
			ContainerPath: "/src/bar",
		},
	}

	expected := []BoiledStep{BoiledStep{cmd: model.ToShellCmd("echo world"), pathMapping: pathMappings[1]}}

	actual, err := BoilSteps(steps, pathMappings)
	if err != nil {
		t.Fatal(err)
	}

	assert.ElementsMatch(t, expected, actual)
}
