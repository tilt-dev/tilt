package ignore

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

type FakeTarget struct {
	path         string
	dockerignore string
}

func (t FakeTarget) LocalRepos() []model.LocalGitRepo {
	return []model.LocalGitRepo{
		model.LocalGitRepo{LocalPath: t.path},
	}
}

func (t FakeTarget) Dockerignores() []model.Dockerignore {
	return []model.Dockerignore{
		model.Dockerignore{
			LocalPath: t.path,
			Contents:  t.dockerignore,
		},
	}
}

func (t FakeTarget) TiltFilename() string {
	return filepath.Join(t.path, "Tiltfile")
}

func (t FakeTarget) IgnoredLocalDirectories() []string {
	return nil
}

type ignoreTestCase struct {
	target               FakeTarget
	change               string
	ignoreInBuildContext bool
	ignoreInFileChange   bool
}

func TestIgnores(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()

	target := FakeTarget{
		path: f.Path(),
	}
	targetWithIgnores := FakeTarget{
		path:         f.Path(),
		dockerignore: "**/ignored.txt",
	}

	cases := []ignoreTestCase{
		{
			target:               target,
			change:               "x.txt",
			ignoreInBuildContext: false,
			ignoreInFileChange:   false,
		},
		{
			target:               target,
			change:               ".git/index",
			ignoreInBuildContext: true,
			ignoreInFileChange:   true,
		},
		{
			target:               target,
			change:               "ignored.txt",
			ignoreInBuildContext: false,
			ignoreInFileChange:   false,
		},
		{
			target:               targetWithIgnores,
			change:               "x.txt",
			ignoreInBuildContext: false,
			ignoreInFileChange:   false,
		},
		{
			target:               targetWithIgnores,
			change:               "ignored.txt",
			ignoreInBuildContext: true,
			ignoreInFileChange:   true,
		},
		{
			target:               target,
			change:               "dir/my-machine.yaml___jb_old___",
			ignoreInBuildContext: false,
			ignoreInFileChange:   true,
		},
		{
			target:               target,
			change:               "dir/.my-machine.yaml.swp",
			ignoreInBuildContext: false,
			ignoreInFileChange:   true,
		},
		{
			target:               target,
			change:               "dir/.my-machine.yaml.swx",
			ignoreInBuildContext: false,
			ignoreInFileChange:   true,
		},
		{
			target:               target,
			change:               "dir/.#my-machine.yaml.swx",
			ignoreInBuildContext: false,
			ignoreInFileChange:   true,
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("TestIgnores%d", i), func(t *testing.T) {
			target := c.target
			change := filepath.Join(f.Path(), c.change)

			ctxFilter := CreateBuildContextFilter(target)
			actual, err := ctxFilter.Matches(change)
			if err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, c.ignoreInBuildContext, actual)

			changeFilter, err := CreateFileChangeFilter(target)
			if err != nil {
				t.Fatal(err)
			}

			actual, err = changeFilter.Matches(change)
			if err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, c.ignoreInFileChange, actual)
		})
	}
}
