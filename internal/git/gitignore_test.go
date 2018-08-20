package git

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/testutils"
)

func TestGitIgnoreTester_Simple(t *testing.T) {
	tf := newTestFixture(t, ".*.swp")
	defer tf.TearDown()

	g, err := NewGitIgnoreTester(tf.tempDirs[0].Path())
	if err != nil {
		t.Error(err)
		return
	}

	ret, err := g.IsIgnored(tf.tempDirs[0].JoinPath("a", "b", ".foo.swp"), false)
	assert.Nil(t, err)
	assert.True(t, ret)
}

func TestGitIgnoreTester_FileOutsideOfRepo(t *testing.T) {
	tf := newTestFixture(t, ".*.swp")
	defer tf.TearDown()

	g, err := NewGitIgnoreTester(tf.tempDirs[0].Path())
	if err != nil {
		t.Error(err)
		return
	}

	ret, err := g.IsIgnored("/tmp/.foo.swp", false)
	assert.Nil(t, err)
	assert.False(t, ret)
}

func TestGitIgnoreTester_GitDirIsIgnored(t *testing.T) {
	tf := newTestFixture(t, ".*.swp")
	defer tf.TearDown()

	g, err := NewRepoIgnoreTester(tf.tempDirs[0].Path())
	if err != nil {
		t.Error(err)
		return
	}

	ret, err := g.IsIgnored(tf.tempDirs[0].JoinPath(".git/foo/bar"), false)
	assert.Nil(t, err)
	assert.True(t, ret)
}

func TestNewMultiRepoIgnoreTester(t *testing.T) {
	tf := newTestFixture(t, ".*.swp", "a.out")
	defer tf.TearDown()

	g, err := NewMultiRepoIgnoreTester([]string{tf.tempDirs[0].Path(), tf.tempDirs[1].Path()})
	if err != nil {
		t.Error(err)
		return
	}

	ret, err := g.IsIgnored(tf.tempDirs[0].JoinPath(".git/foo/bar"), false)
	assert.Nil(t, err)
	assert.True(t, ret)

	ret, err = g.IsIgnored(tf.tempDirs[0].JoinPath(".foo.swp"), false)
	assert.Nil(t, err)
	assert.True(t, ret)

	ret, err = g.IsIgnored(tf.tempDirs[1].JoinPath("a.out"), false)
	assert.Nil(t, err)
	assert.True(t, ret)

	ret, err = g.IsIgnored(tf.tempDirs[1].JoinPath(".foo.swp"), false)
	assert.Nil(t, err)
	assert.False(t, ret)

}

type testFixture struct {
	tempDirs []*testutils.TempDirFixture
}

// initializes `tf.tempDirs` to be an array with one dir per gitignore
func newTestFixture(t *testing.T, gitignores ...string) *testFixture {
	tf := testFixture{}
	for _, gitignore := range gitignores {
		tempDir := testutils.NewTempDirFixture(t)
		f, err := os.Create(tempDir.JoinPath(".gitignore"))
		if err != nil {
			t.Error(err)
			return nil
		}
		f.WriteString(gitignore)
		f.Close()
		tf.tempDirs = append(tf.tempDirs, tempDir)
	}
	return &tf
}

func (tf *testFixture) TearDown() {
	for _, tempDir := range tf.tempDirs {
		tempDir.TearDown()
	}
}
