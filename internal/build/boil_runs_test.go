package build

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/pkg/model"
)

func TestBoilRunsNoTrigger(t *testing.T) {
	runs := []model.Run{
		model.Run{
			Cmd: model.ToUnixCmd("echo hello"),
		},
	}

	pathMappings := []PathMapping{
		PathMapping{
			LocalPath:     AbsPath("test", "foo"),
			ContainerPath: "/src/foo",
		},
	}

	expected := []model.Cmd{model.ToUnixCmd("echo hello")}

	actual, err := BoilRuns(runs, pathMappings)
	if err != nil {
		t.Fatal(err)
	}

	assert.ElementsMatch(t, expected, actual)
}

func TestBoilRunsNoFilesChanged(t *testing.T) {
	runs := []model.Run{
		model.Run{
			Cmd: model.ToUnixCmd("echo hello"),
		},
	}

	pathMappings := []PathMapping{}

	expected := []model.Cmd{model.ToUnixCmd("echo hello")}

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
			Cmd:      model.ToUnixCmd("echo hello"),
			Triggers: model.NewPathSet(triggers, AbsPath("test")),
		},
	}

	pathMappings := []PathMapping{
		PathMapping{
			LocalPath:     AbsPath("test", "foo"),
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
			Cmd:      model.ToUnixCmd("echo world"),
			Triggers: model.NewPathSet(triggers, AbsPath("test")),
		},
	}

	pathMappings := []PathMapping{
		PathMapping{
			LocalPath:     AbsPath("test", "bar"),
			ContainerPath: "/src/bar",
		},
	}

	expected := []model.Cmd{model.ToUnixCmd("echo world")}

	actual, err := BoilRuns(runs, pathMappings)
	if err != nil {
		t.Fatal(err)
	}

	assert.ElementsMatch(t, expected, actual)
}

func TestBoilRunsTriggerMatchingAbsPath(t *testing.T) {
	triggers := []string{AbsPath("test", "bar")}
	runs := []model.Run{
		model.Run{
			Cmd:      model.ToUnixCmd("echo world"),
			Triggers: model.NewPathSet(triggers, AbsPath("test")),
		},
	}

	pathMappings := []PathMapping{
		PathMapping{
			LocalPath:     AbsPath("test", "bar"),
			ContainerPath: "/src/bar",
		},
	}

	expected := []model.Cmd{model.ToUnixCmd("echo world")}

	actual, err := BoilRuns(runs, pathMappings)
	if err != nil {
		t.Fatal(err)
	}

	assert.ElementsMatch(t, expected, actual)
}

func TestBoilRunsTriggerNestedPathNoMatch(t *testing.T) {
	triggers := []string{"bar"}
	runs := []model.Run{
		model.Run{
			Cmd:      model.ToUnixCmd("echo world"),
			Triggers: model.NewPathSet(triggers, AbsPath("test")),
		},
	}

	pathMappings := []PathMapping{
		PathMapping{
			LocalPath:     "/home/tilt/code/test/nested/bar",
			ContainerPath: "/src/bar",
		},
	}

	expected := []model.Cmd{}

	actual, err := BoilRuns(runs, pathMappings)
	if err != nil {
		t.Fatal(err)
	}

	assert.ElementsMatch(t, expected, actual)
}

func TestBoilRunsManyTriggersManyFiles(t *testing.T) {
	wd := AbsPath("test")
	triggers1 := []string{"foo"}
	triggers2 := []string{"bar"}
	runs := []model.Run{
		model.Run{
			Cmd:      model.ToUnixCmd("echo hello"),
			Triggers: model.NewPathSet(triggers1, wd),
		},
		model.Run{
			Cmd:      model.ToUnixCmd("echo world"),
			Triggers: model.NewPathSet(triggers2, wd),
		},
	}

	pathMappings := []PathMapping{
		PathMapping{
			LocalPath:     AbsPath("test", "baz"),
			ContainerPath: "/src/baz",
		},
		PathMapping{
			LocalPath:     AbsPath("test", "bar"),
			ContainerPath: "/src/bar",
		},
	}

	expected := []model.Cmd{model.ToUnixCmd("echo world")}

	actual, err := BoilRuns(runs, pathMappings)
	if err != nil {
		t.Fatal(err)
	}

	assert.ElementsMatch(t, expected, actual)
}

func AbsPath(parts ...string) string {
	if runtime.GOOS == "windows" {
		return filepath.Join(append([]string{"C:\\home\\tilt"}, parts...)...)
	}
	return filepath.Join(append([]string{"/home/tilt"}, parts...)...)
}
