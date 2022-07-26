package ignore

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
)

type FakeTarget struct {
	path                 string
	dockerignorePatterns []string
}

func (t FakeTarget) GetIgnores() []v1alpha1.IgnoreDef {
	result := []v1alpha1.IgnoreDef{
		{BasePath: filepath.Join(t.path, "Tiltfile")},
		{BasePath: filepath.Join(t.path, ".git")},
	}

	if len(t.dockerignorePatterns) != 0 {
		result = append(result, v1alpha1.IgnoreDef{BasePath: t.path, Patterns: t.dockerignorePatterns})
	}
	return result
}

type ignoreTestCase struct {
	target               FakeTarget
	change               string
	ignoreInBuildContext bool
	ignoreInFileChange   bool
}

func TestIgnores(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)

	target := FakeTarget{
		path: f.Path(),
	}
	targetWithIgnores := FakeTarget{
		path:                 f.Path(),
		dockerignorePatterns: []string{"**/ignored.txt"},
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
			change:               "dir/.my-machine.yaml.swn",
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
		{
			target:               target,
			change:               "dir/#my-machine#",
			ignoreInBuildContext: false,
			ignoreInFileChange:   true,
		},
	}

	for i, c := range cases {
		t.Run(fmt.Sprintf("TestIgnores%d", i), func(t *testing.T) {
			target := c.target
			change := filepath.Join(f.Path(), c.change)

			ctxFilter := CreateBuildContextFilter(target.GetIgnores())
			actual, err := ctxFilter.Matches(change)
			if err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, c.ignoreInBuildContext, actual)

			changeFilter := CreateFileChangeFilter(target.GetIgnores())
			actual, err = changeFilter.Matches(change)
			if err != nil {
				t.Fatal(err)
			}

			assert.Equal(t, c.ignoreInFileChange, actual)
		})
	}
}
