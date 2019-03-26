package build

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/model"
)

func TestBoilRunsNoTrigger(t *testing.T) {
	runs := []model.Run{
		model.Run{
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

	actual, err := BoilRuns(runs, pathMappings)
	if err != nil {
		t.Fatal(err)
	}

	assert.ElementsMatch(t, expected, actual)
}

func TestBoilRunsNoFilesChanged(t *testing.T) {
	runs := []model.Run{
		model.Run{
			Cmd: model.ToShellCmd("echo hello"),
		},
	}

	pathMappings := []PathMapping{}

	expected := []model.Cmd{model.ToShellCmd("echo hello")}

	actual, err := BoilRuns(runs, pathMappings)
	if err != nil {
		t.Fatal(err)
	}

	assert.ElementsMatch(t, expected, actual)
}

func TestBoilRunsOneTriggerFilesDontMatch(t *testing.T) {
	triggers := []string{"bar"}
	runs := []model.Run{
		model.Run{
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

	actual, err := BoilRuns(runs, pathMappings)
	if err != nil {
		t.Fatal(err)
	}

	assert.ElementsMatch(t, expected, actual)
}

func TestBoilRunsOneTriggerMatchingFile(t *testing.T) {
	triggers := []string{"bar"}
	runs := []model.Run{
		model.Run{
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

	actual, err := BoilRuns(runs, pathMappings)
	if err != nil {
		t.Fatal(err)
	}

	assert.ElementsMatch(t, expected, actual)
}

func TestBoilRunsManyTriggersManyFiles(t *testing.T) {
	wd := "/home/tilt/code/test"
	triggers1 := []string{"foo"}
	triggers2 := []string{"bar"}
	runs := []model.Run{
		model.Run{
			Cmd:           model.ToShellCmd("echo hello"),
			Triggers:      triggers1,
			BaseDirectory: wd,
		},
		model.Run{
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

	actual, err := BoilRuns(runs, pathMappings)
	if err != nil {
		t.Fatal(err)
	}

	assert.ElementsMatch(t, expected, actual)
}
