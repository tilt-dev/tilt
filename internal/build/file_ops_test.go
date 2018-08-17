package build

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils"
)

func TestFilesToOps(t *testing.T) {
	f := newFileOpsFixture(t)
	defer f.TearDown()

	paths := []string{
		"mount1/fileA",
		"mount1/child/fileB",
		"mount2/fileC",
		"notAMount/fileD",
	}
	f.TouchFiles(paths)

	absPaths := make([]string, len(paths))
	for i, p := range paths {
		absPaths[i] = filepath.Join(f.Path(), p)
	}

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
	ops, err := FilesToOps(f.ctx, absPaths, mounts)
	if err != nil {
		t.Fatal(err)
	}

	expected := []FileOp{
		FileOp{
			LocalPath:     filepath.Join(f.Path(), "mount1/fileA"),
			ContainerPath: "/dest1/fileA",
		},
		FileOp{
			LocalPath:     filepath.Join(f.Path(), "mount1/child/fileB"),
			ContainerPath: "/dest1/child/fileB",
		},
		FileOp{
			LocalPath:     filepath.Join(f.Path(), "mount2/fileC"),
			ContainerPath: "/nested/dest2/fileC",
		},
	}

	assert.ElementsMatch(t, expected, ops)
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
		ctx:            context.Background(),
	}
}
