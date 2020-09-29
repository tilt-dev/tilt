package model

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/tilt-dev/tilt/internal/ospath"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
)

func TestNewRelativeFileOrChildMatcher(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()

	paths := []string{
		"a",
		"b/c/d",
		ospath.MustAbs("already/abs"),
	}
	matcher := NewRelativeFileOrChildMatcher(f.Path(), paths...)

	expected := map[string]bool{
		f.JoinPath("a"):               true,
		f.JoinPath("b/c/d"):           true,
		ospath.MustAbs("already/abs"): true,
	}

	assert.Equal(t, expected, matcher.paths)
}

func TestFileOrChildMatcher(t *testing.T) {
	f := tempdir.NewTempDirFixture(t)
	defer f.TearDown()

	matcher := fileOrChildMatcher{map[string]bool{
		"file.txt":        true,
		"nested/file.txt": true,
		"directory":       true,
	}}

	// map test case --> expected match
	expectedMatch := map[string]bool{
		"file.txt":                true,
		"nested/file.txt":         true,
		"nested":                  false,
		"nested/otherfile.txt":    false,
		"directory/some/file.txt": true,
		"other/dir/entirely":      false,
	}

	for f, expected := range expectedMatch {
		match, err := matcher.Matches(f)
		if assert.NoError(t, err) {
			assert.Equal(t, expected, match, "expected file '%s' match --> %t", f, expected)
		}
	}
}
