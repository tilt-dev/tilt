package build

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

func TestFilesToPathMappings(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
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
			LocalPath:     f.JoinPath("mount1"),
			ContainerPath: "/dest1",
		},
		model.Mount{
			LocalPath:     f.JoinPath("mount2"),
			ContainerPath: "/nested/dest2",
		},
	}
	actual, err := FilesToPathMappings(absPaths, mounts)
	if err != nil {
		f.T().Fatal(err)
	}

	expected := []PathMapping{
		PathMapping{
			LocalPath:     filepath.Join(f.Path(), "mount1/fileA"),
			ContainerPath: "/dest1/fileA",
		},
		PathMapping{
			LocalPath:     filepath.Join(f.Path(), "mount1/child/fileB"),
			ContainerPath: "/dest1/child/fileB",
		},
		PathMapping{
			LocalPath:     filepath.Join(f.Path(), "mount2/fileC"),
			ContainerPath: "/nested/dest2/fileC",
		},
		PathMapping{
			LocalPath:     filepath.Join(f.Path(), "mount2/file_deleted"),
			ContainerPath: "/nested/dest2/file_deleted",
		},
	}

	assert.ElementsMatch(t, expected, actual)
}

func TestFileToDirectoryPathMapping(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()

	paths := []string{
		"mount1/fileA",
	}
	f.TouchFiles(paths)

	absPaths := make([]string, len(paths))
	for i, p := range paths {
		absPaths[i] = f.JoinPath(p)
	}

	mounts := []model.Mount{
		model.Mount{
			LocalPath:     f.JoinPath("mount1", "fileA"),
			ContainerPath: "/dest1/",
		},
	}

	actual, err := FilesToPathMappings(absPaths, mounts)
	if err != nil {
		f.T().Fatal(err)
	}

	expected := []PathMapping{
		PathMapping{
			LocalPath:     filepath.Join(f.Path(), "mount1/fileA"),
			ContainerPath: "/dest1/fileA",
		},
	}

	assert.ElementsMatch(t, expected, actual)
}

func TestFileNotInMountThrowsErr(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()

	files := []string{f.JoinPath("not/a/mount/fileA")}

	mounts := []model.Mount{
		model.Mount{
			LocalPath:     f.JoinPath("mount1"),
			ContainerPath: "/dest1",
		},
	}

	_, err := FilesToPathMappings(files, mounts)
	if assert.NotNil(t, err, "expected error for file not matching any mounts") {
		assert.Contains(t, err.Error(), "matches no mounts")
	}
}
