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

	pathMappings := []PathMapping{
		PathMapping{
			LocalPath:     "/home/tilt/code/test/foo",
			ContainerPath: "/src/foo",
		},
	}

	expected := []model.Cmd{model.ToShellCmd("echo hello")}

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

	pathMappings := []PathMapping{}

	expected := []model.Cmd{model.ToShellCmd("echo hello")}

	actual, err := BoilSteps(steps, pathMappings)
	if err != nil {
		t.Fatal(err)
	}

	assert.ElementsMatch(t, expected, actual)
}

func TestBoilStepsOneTriggerFilesDontMatch(t *testing.T) {
	triggers := []string{"bar"}
	steps := []model.Step{
		model.Step{
			Cmd:           model.ToShellCmd("echo hello"),
			Triggers:      triggers,
			BaseDirectory: "/home/tilt/code/test",
		},
	}

	pathMappings := []PathMapping{
		PathMapping{
			LocalPath:     "/home/tilt/code/test/foo",
			ContainerPath: "/src/foo",
		},
	}

	expected := []model.Cmd{}

	actual, err := BoilSteps(steps, pathMappings)
	if err != nil {
		t.Fatal(err)
	}

	assert.ElementsMatch(t, expected, actual)
}

func TestBoilStepsOneTriggerMatchingFile(t *testing.T) {
	triggers := []string{"bar"}
	steps := []model.Step{
		model.Step{
			Cmd:           model.ToShellCmd("echo world"),
			Triggers:      triggers,
			BaseDirectory: "/home/tilt/code/test",
		},
	}

	pathMappings := []PathMapping{
		PathMapping{
			LocalPath:     "/home/tilt/code/test/bar",
			ContainerPath: "/src/bar",
		},
	}

	expected := []model.Cmd{model.ToShellCmd("echo world")}

	actual, err := BoilSteps(steps, pathMappings)
	if err != nil {
		t.Fatal(err)
	}

	assert.ElementsMatch(t, expected, actual)
}

func TestBoilStepsManyTriggersManyFiles(t *testing.T) {
	wd := "/home/tilt/code/test"
	triggers1 := []string{"foo"}
	triggers2 := []string{"bar"}
	steps := []model.Step{
		model.Step{
			Cmd:           model.ToShellCmd("echo hello"),
			Triggers:      triggers1,
			BaseDirectory: wd,
		},
		model.Step{
			Cmd:           model.ToShellCmd("echo world"),
			Triggers:      triggers2,
			BaseDirectory: wd,
		},
	}

	pathMappings := []PathMapping{
		PathMapping{
			LocalPath:     "/home/tilt/code/test/baz",
			ContainerPath: "/src/baz",
		},
		PathMapping{
			LocalPath:     "/home/tilt/code/test/bar",
			ContainerPath: "/src/bar",
		},
	}

	expected := []model.Cmd{model.ToShellCmd("echo world")}

	actual, err := BoilSteps(steps, pathMappings)
	if err != nil {
		t.Fatal(err)
	}

	assert.ElementsMatch(t, expected, actual)
}
