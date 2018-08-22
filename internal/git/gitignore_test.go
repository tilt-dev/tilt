package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/testutils"
)

func TestGitIgnoreTester_Simple(t *testing.T) {
	tf := newTestFixture(t, ".*.swp")
	defer tf.TearDown()

	g, err := NewGitIgnoreTester(testutils.CtxForTest(), tf.repoRoots[0].Path())
	if err != nil {
		t.Fatal(err)
	}

	ret, err := g.IsIgnored(tf.repoRoots[0].JoinPath("a", "b", ".foo.swp"), false)
	if assert.NoError(t, err) {
		assert.True(t, ret)
	}
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

	g, err := NewGitIgnoreTester(testutils.CtxForTest(), tf.repoRoots[0].Path())
	if err != nil {
		t.Error(err)
		return
	}

	ret, err := g.IsIgnored("/tmp/.foo.swp", false)
	if assert.NoError(t, err) {
		assert.False(t, ret)
	}
}

func TestGitIgnoreTester_GitDirIsIgnored(t *testing.T) {
	tf := newTestFixture(t, ".*.swp")
	defer tf.TearDown()

	g, err := NewRepoIgnoreTester(testutils.CtxForTest(), tf.repoRoots[0].Path())
	if err != nil {
		t.Fatal(err)
	}

	ret, err := g.IsIgnored(tf.repoRoots[0].JoinPath(".git/foo/bar"), false)
	if assert.NoError(t, err) {
		assert.True(t, ret)
	}
}

func TestNewMultiRepoIgnoreTester(t *testing.T) {
	tf := newTestFixture(t, ".*.swp", "a.out")
	defer tf.TearDown()

	g, err := NewMultiRepoIgnoreTester(testutils.CtxForTest(), []string{tf.repoRoots[0].Path(), tf.repoRoots[1].Path()})
	if err != nil {
		t.Fatal(err)
	}

	ret, err := g.IsIgnored(tf.repoRoots[0].JoinPath(".git/foo/bar"), false)
	if assert.NoError(t, err) {
		assert.True(t, ret)
	}

	ret, err = g.IsIgnored(tf.repoRoots[0].JoinPath(".foo.swp"), false)
	if assert.NoError(t, err) {
		assert.True(t, ret)
	}

	ret, err = g.IsIgnored(tf.repoRoots[1].JoinPath("a.out"), false)
	if assert.NoError(t, err) {
		assert.True(t, ret)
	}

	ret, err = g.IsIgnored(tf.repoRoots[1].JoinPath(".foo.swp"), false)
	if assert.NoError(t, err) {
		assert.False(t, ret)
	}

}

type testFixture struct {
	repoRoots []*testutils.TempDirFixture
}

// initializes `tf.repoRoots` to be an array with one dir per gitignore
func newTestFixture(t *testing.T, gitignores ...string) *testFixture {
	tf := testFixture{}
	for _, gitignore := range gitignores {
		tempDir := testutils.NewTempDirFixture(t)
		tempDir.WriteFile(".gitignore", gitignore)
		tf.repoRoots = append(tf.repoRoots, tempDir)
	}
	return &tf
}

func (tf *testFixture) TearDown() {
	for _, tempDir := range tf.repoRoots {
		tempDir.TearDown()
	}
}
