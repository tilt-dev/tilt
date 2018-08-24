package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils"
)

func TestFilesToPathMappings(t *testing.T) {
	f := testutils.NewTempDirFixture(t)
	defer f.TearDown()

	paths := []string{
		"mount1/fileA",
		"mount1/child/fileB",
		"mount2/fileC",
	}
	f.TouchFiles(paths)

	absPaths := make([]string, len(paths))
	for i, p := range paths {
		absPaths[i] = f.JoinPath(p)
	}
	// Add a file that doesn't exist on local -- but we still expect it to successfully
	// map to a ContainerPath.
	absPaths = append(absPaths, filepath.Join(f.Path(), "mount2/file_deleted"))

	mounts := []model.Mount{
		model.Mount{
			Repo:          model.LocalGithubRepo{LocalPath: f.JoinPath("mount1")},
			ContainerPath: "/dest1",
		},
		model.Mount{
			Repo:          model.LocalGithubRepo{LocalPath: f.JoinPath("mount2")},
			ContainerPath: "/nested/dest2",
		},
	}
	actual, err := FilesToPathMappings(absPaths, mounts)
	if err != nil {
		f.T().Fatal(err)
	}

	expected := []pathMapping{
		pathMapping{
			LocalPath:     filepath.Join(f.Path(), "mount1/fileA"),
			ContainerPath: "/dest1/fileA",
		},
		pathMapping{
			LocalPath:     filepath.Join(f.Path(), "mount1/child/fileB"),
			ContainerPath: "/dest1/child/fileB",
		},
		pathMapping{
			LocalPath:     filepath.Join(f.Path(), "mount2/fileC"),
			ContainerPath: "/nested/dest2/fileC",
		},
		pathMapping{
			LocalPath:     filepath.Join(f.Path(), "mount2/file_deleted"),
			ContainerPath: "/nested/dest2/file_deleted",
		},
	}

	assert.ElementsMatch(t, expected, actual)
}

func TestRelativeRepoPathsBecomeAbsolute(t *testing.T) {
	f := testutils.NewTempDirFixture(t)
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldWd)
	err = os.Chdir(f.Path())
	if err != nil {
		t.Fatal(err)
	}
	defer f.TearDown()

	files := []string{filepath.Join(f.Path(), "timing_script-W3imtct67O")}
	mounts := []model.Mount{
		model.Mount{
			Repo:          model.LocalGithubRepo{LocalPath: "."},
			ContainerPath: "/go/src/github.com/windmilleng/blorgly-backend",
		},
	}

	actual, err := FilesToPathMappings(files, mounts)
	if err != nil {
		t.Fatal(err)
	}

	expected := []pathMapping{
		pathMapping{
			LocalPath:     filepath.Join(f.Path(), "timing_script-W3imtct67O"),
			ContainerPath: "/go/src/github.com/windmilleng/blorgly-backend/timing_script-W3imtct67O",
		},
	}

	assert.ElementsMatch(t, expected, actual)
}

func TestFileNotInMountThrowsErr(t *testing.T) {
	f := testutils.NewTempDirFixture(t)
	defer f.TearDown()

	files := []string{f.JoinPath("not/a/mount/fileA")}

	mounts := []model.Mount{
		model.Mount{
			Repo:          model.LocalGithubRepo{LocalPath: f.JoinPath("mount1")},
			ContainerPath: "/dest1",
		},
	}

	_, err := FilesToPathMappings(files, mounts)
	if assert.NotNil(t, err, "expected error for file not matching any mounts") {
		assert.Contains(t, err.Error(), "matches no mounts")
	}
}
