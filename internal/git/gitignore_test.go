package git

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/ignore"
	"github.com/windmilleng/tilt/internal/testutils"
)

func TestGitIgnoreTester_Simple(t *testing.T) {
	tf := newTestFixture(t, ".*.swp")
	defer tf.TearDown()

	tf.UseGitIgnoreTester()

	tf.AssertResult(tf.JoinPath(0, "a", "b", ".foo.swp"), true, false)
}

func TestNewGitIgnoreTester_NoGitignore(t *testing.T) {
	tempDir := testutils.NewTempDirFixture(t)
	defer tempDir.TearDown()

	g, err := NewGitIgnoreTester(testutils.CtxForTest(), tempDir.Path())
	if err != nil {
		t.Fatal(err)
	}

	// we were really just looking for a lack of error on initialization
	ret, err := g.IsIgnored(tempDir.JoinPath("a", "b", ".foo.swp"), false)
	assert.Nil(t, err)
	assert.False(t, ret)

	ret, err = g.IsIgnored(tempDir.JoinPath("foo.txt"), false)
	if assert.NoError(t, err) {
		assert.False(t, ret)
	}

}

func TestGitIgnoreTester_FileOutsideOfRepo(t *testing.T) {
	tf := newTestFixture(t, ".*.swp")
	defer tf.TearDown()

	tf.UseSingleRepoTester()
	tf.AssertResult(filepath.Join("/tmp", ".foo.swp"), false, false)
}

func TestGitIgnoreTester_GitDirIsIgnored(t *testing.T) {
	tf := newTestFixture(t, ".*.swp")
	defer tf.TearDown()

	tf.UseSingleRepoTester()
	tf.AssertResult(tf.JoinPath(0, ".git", "foo", "bar"), true, false)
}

func TestNewMultiRepoIgnoreTester(t *testing.T) {
	tf := newTestFixture(t, ".*.swp", "a.out")
	defer tf.TearDown()

	tf.UseMultiRepoTester()

	tf.AssertResult(tf.JoinPath(0, ".git", "foo", "bar"), true, false)
	tf.AssertResult(tf.JoinPath(0, ".foo.swp"), true, false)
	tf.AssertResult(tf.JoinPath(1, "a.out"), true, false)
	tf.AssertResult(tf.JoinPath(1, ".foo.swp"), false, false)
}

func TestRepoIgnoreTester_IsIgnoredRelativePath(t *testing.T) {
	tf := newTestFixture(t, "")
	defer tf.TearDown()

}

type testFixture struct {
	repoRoots []*testutils.TempDirFixture
	tester    ignore.Tester
	ctx       context.Context
	t         *testing.T
}

// initializes `tf.repoRoots` to be an array with one dir per gitignore
func newTestFixture(t *testing.T, gitignores ...string) *testFixture {
	tf := testFixture{}
	for _, gitignore := range gitignores {
		tempDir := testutils.NewTempDirFixture(t)
		tempDir.WriteFile(".gitignore", gitignore)
		tf.repoRoots = append(tf.repoRoots, tempDir)
	}

	tf.ctx = context.Background()
	tf.t = t
	return &tf
}

func (tf *testFixture) UseGitIgnoreTester() {
	tester, err := NewGitIgnoreTester(testutils.CtxForTest(), tf.repoRoots[0].Path())
	if err != nil {
		tf.t.Fatal(err)
	}

	tf.tester = tester
}

func (tf *testFixture) UseSingleRepoTester() {
	tf.UseSingleRepoTesterWithPath(tf.repoRoots[0].Path())
}

func (tf *testFixture) UseSingleRepoTesterWithPath(path string) {
	tester, err := NewRepoIgnoreTester(tf.ctx, path)
	if err != nil {
		tf.t.Fatal(err)
	}
	tf.tester = tester
}

func (tf *testFixture) UseMultiRepoTester() {
	var rootDirs []string
	for _, dir := range tf.repoRoots {
		rootDirs = append(rootDirs, dir.Path())
	}

	tester, err := NewMultiRepoIgnoreTester(tf.ctx, rootDirs)
	if err != nil {
		tf.t.Fatal(err)
	}

	tf.tester = tester
}

func (tf *testFixture) JoinPath(repoNum int, path ...string) string {
	return tf.repoRoots[repoNum].JoinPath(path...)
}

func (tf *testFixture) AssertResult(path string, expectedIsIgnored bool, expectError bool) {
	tf.t.Helper()
	isIgnored, err := tf.tester.IsIgnored(path, false)
	if expectError {
		assert.Error(tf.t, err)
	} else {
		if assert.NoError(tf.t, err) {
			assert.Equal(tf.t, expectedIsIgnored, isIgnored)
		}
	}
}

func (tf *testFixture) TearDown() {
	for _, tempDir := range tf.repoRoots {
		tempDir.TearDown()
	}
}
