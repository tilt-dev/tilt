package dockerignore_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/windmilleng/tilt/internal/dockerignore"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

func TestMatches(t *testing.T) {
	tf := newTestFixture(t, "node_modules")
	defer tf.TearDown()
	tf.AssertResult(tf.JoinPath("node_modules", "foo"), true)
	tf.AssertResult(tf.JoinPath("foo", "bar"), false)
}

func TestPatterns(t *testing.T) {
	tf := newTestFixture(t, "node_modules")
	defer tf.TearDown()

	patterns := tf.tester.(model.PatternMatcher).AsMatchPatterns()
	assert.Equal(t, []string{"node_modules"}, patterns)
}

func TestComment(t *testing.T) {
	tf := newTestFixture(t, "# generated code")
	defer tf.TearDown()
	tf.AssertResult(tf.JoinPath("node_modules", "foo"), false)
	tf.AssertResult(tf.JoinPath("foo", "bar"), false)
}

func TestGlob(t *testing.T) {
	tf := newTestFixture(t, "*/temp*")
	defer tf.TearDown()
	tf.AssertResult(tf.JoinPath("somedir", "temporary.txt"), true)
	tf.AssertResult(tf.JoinPath("somedir", "temp"), true)
}

func TestOneCharacterExtension(t *testing.T) {
	tf := newTestFixture(t, "temp?")
	defer tf.TearDown()
	tf.AssertResult(tf.JoinPath("tempa"), true)
	tf.AssertResult(tf.JoinPath("tempeh"), false)
	tf.AssertResult(tf.JoinPath("temp"), false)
}

func TestException(t *testing.T) {
	tf := newTestFixture(t, "docs", "!docs/README.md")
	defer tf.TearDown()
	tf.AssertResult(tf.JoinPath("docs", "stuff.md"), true)
	tf.AssertResult(tf.JoinPath("docs", "README.md"), false)
}

func TestNoDockerignoreFile(t *testing.T) {
	tf := newTestFixture(t)
	defer tf.TearDown()
	tf.AssertResult(tf.JoinPath("hi"), false)
	tf.AssertResult(tf.JoinPath("hi", "hello"), false)
}

type testFixture struct {
	repoRoot *tempdir.TempDirFixture
	t        *testing.T
	tester   model.PathMatcher
}

func newTestFixture(t *testing.T, dockerignores ...string) *testFixture {
	tf := testFixture{}
	tempDir := tempdir.NewTempDirFixture(t)
	tf.repoRoot = tempDir
	ignoreText := strings.Builder{}
	for _, rule := range dockerignores {
		ignoreText.WriteString(rule + "\n")
	}
	if ignoreText.Len() > 0 {
		tempDir.WriteFile(".dockerignore", ignoreText.String())
	}

	tester, err := dockerignore.NewDockerIgnoreTester(tempDir.Path())
	if err != nil {
		t.Fatal(err)
	}
	tf.tester = tester

	tf.t = t
	return &tf
}

func (tf *testFixture) JoinPath(path ...string) string {
	return tf.repoRoot.JoinPath(path...)
}

func (tf *testFixture) AssertResult(path string, expectedMatches bool) {
	isIgnored, err := tf.tester.Matches(path, false)
	if err != nil {
		tf.t.Fatal(err)
	} else {
		if assert.NoError(tf.t, err) {
			assert.Equalf(tf.t, expectedMatches, isIgnored, "Expected isIgnored to be %t for file %s, got %t", expectedMatches, path, isIgnored)
		}
	}
}

func (tf *testFixture) TearDown() {
	tf.repoRoot.TearDown()
}
