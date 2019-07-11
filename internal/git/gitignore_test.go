package git_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/windmilleng/tilt/internal/git"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

func TestGitIgnoreTester_GitDirMatches(t *testing.T) {
	tf := newTestFixture(t)
	defer tf.TearDown()

	tests := []struct {
		description string
		path        []string
		expectMatch bool
		expectError bool
	}{
		{
			description: "a file in the .git directory",
			path:        []string{".git", "foo", "bar"},
			expectMatch: true,
			expectError: false,
		},
		{
			description: "a .gitlab-ci.yml file",
			path:        []string{".gitlab-ci.yml"},
			expectMatch: false,
			expectError: false,
		},
		{
			description: "a foo.git file",
			path:        []string{"foo.git"},
			expectMatch: false,
			expectError: false,
		},
	}

	for _, tt := range tests {
		tf.AssertResult(tt.description, tf.JoinPath(0, tt.path...), tt.expectMatch, tt.expectError)
	}
}

type testFixture struct {
	repoRoots []*tempdir.TempDirFixture
	tester    model.PathMatcher
	ctx       context.Context
	t         *testing.T
}

// initializes `tf.repoRoots` to be an array with one dir per gitignore
func newTestFixture(t *testing.T) *testFixture {
	tf := testFixture{}
	tf.repoRoots = append(tf.repoRoots, tempdir.NewTempDirFixture(t))
	tf.ctx = context.Background()
	tf.t = t
	tf.UseSingleRepoTester()
	return &tf
}

func (tf *testFixture) UseSingleRepoTester() {
	tf.UseSingleRepoTesterWithPath(tf.repoRoots[0].Path())
}

func (tf *testFixture) UseSingleRepoTesterWithPath(path string) {
	tf.tester = git.NewRepoIgnoreTester(tf.ctx, path)
}

func (tf *testFixture) JoinPath(repoNum int, path ...string) string {
	return tf.repoRoots[repoNum].JoinPath(path...)
}

func (tf *testFixture) AssertResult(description, path string, expectedMatches bool, expectError bool) {
	tf.t.Run(description, func(t *testing.T) {
		isIgnored, err := tf.tester.Matches(path)
		if expectError {
			assert.Error(t, err)
		} else {
			if assert.NoError(t, err) {
				assert.Equal(t, expectedMatches, isIgnored)
			}
		}
	})
}

func (tf *testFixture) TearDown() {
	for _, tempDir := range tf.repoRoots {
		tempDir.TearDown()
	}
}
