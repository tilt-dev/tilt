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

	tf.AssertResult(tf.JoinPath(0, ".git", "foo", "bar"), true, false)
	tf.AssertResult(tf.JoinPath(0, ".gitlab-ci.yml"), false, false)
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
	tester, err := git.NewRepoIgnoreTester(tf.ctx, path)
	if err != nil {
		tf.t.Fatal(err)
	}
	tf.tester = tester
}

func (tf *testFixture) JoinPath(repoNum int, path ...string) string {
	return tf.repoRoots[repoNum].JoinPath(path...)
}

func (tf *testFixture) AssertResult(path string, expectedMatches bool, expectError bool) {
	isIgnored, err := tf.tester.Matches(path, false)
	if expectError {
		assert.Error(tf.t, err)
	} else {
		if assert.NoError(tf.t, err) {
			assert.Equal(tf.t, expectedMatches, isIgnored)
		}
	}
}

func (tf *testFixture) TearDown() {
	for _, tempDir := range tf.repoRoots {
		tempDir.TearDown()
	}
}
