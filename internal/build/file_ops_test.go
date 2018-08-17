package build

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils"
)

func TestFilesToPathMappings(t *testing.T) {
	f := newFileOpsFixture(t)
	defer f.TearDown()

	paths := []string{
		"mount1/fileA",
		"mount1/child/fileB",
		"mount2/fileC",
	}
	f.TouchFiles(paths)

	absPaths := make([]string, len(paths))
	for i, p := range paths {
		absPaths[i] = filepath.Join(f.Path(), p)
	}
	// Add a file that doesn't exist on local -- but we still expect it to successfully
	// map to a ContainerPath.
	absPaths = append(absPaths, filepath.Join(f.Path(), "mount2/file_deleted"))

	mounts := []model.Mount{
		model.Mount{
			Repo:          model.LocalGithubRepo{LocalPath: filepath.Join(f.Path(), "mount1")},
			ContainerPath: "/dest1",
		},
		model.Mount{
			Repo:          model.LocalGithubRepo{LocalPath: filepath.Join(f.Path(), "mount2")},
			ContainerPath: "/nested/dest2",
		},
	}
	actual, err := FilesToPathMappings(f.ctx, absPaths, mounts)
	if err != nil {
		f.t.Fatal(err)
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

func TestFileNotInMountThrowsErr(t *testing.T) {
	f := newFileOpsFixture(t)
	defer f.TearDown()

	files := []string{filepath.Join(f.Path(), "not/a/mount/fileA")}

	mounts := []model.Mount{
		model.Mount{
			Repo:          model.LocalGithubRepo{LocalPath: filepath.Join(f.Path(), "mount1")},
			ContainerPath: "/dest1",
		},
	}

	_, err := FilesToPathMappings(f.ctx, files, mounts)
	if assert.NotNil(t, err, "expected error for file not matching any mounts") {
		assert.Contains(t, err.Error(), "matches no mounts")
	}
}

type fileOpsFixture struct {
	*testutils.TempDirFixture
	t   *testing.T
	ctx context.Context
}

func newFileOpsFixture(t *testing.T) *fileOpsFixture {
	return &fileOpsFixture{
		TempDirFixture: testutils.NewTempDirFixture(t),
		t:              t,
		ctx:            testutils.CtxForTest(),
	}
}
