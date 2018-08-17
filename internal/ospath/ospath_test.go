package ospath

import (
	"testing"

	"github.com/windmilleng/tilt/internal/testutils"
)

func TestChild(t *testing.T) {
	f := NewOspathFixture(t)
	defer f.TearDown()

	paths := []string{
		"parent/fileA",
		"parent/child/fileB",
		"parent/child/grandchild/fileC",
		"sibling/fileD",
	}
	f.TouchFiles(paths)

	f.assertChild("parent", "sibling/fileD", "")
	f.assertChild("parent/child", "parent/fileA", "")
	f.assertChild("parent", "parent/fileA", "fileA")
	f.assertChild("parent", "parent/child/fileB", "child/fileB")
	f.assertChild("parent", "parent/child/grandchild/fileC", "child/grandchild/fileC")

	f.assertChild("parent/child", "parent/child/fileB", "fileB")
}

type OspathFixture struct {
	*testutils.TempDirFixture
	t *testing.T
}

func NewOspathFixture(t *testing.T) *OspathFixture {
	return &OspathFixture{
		TempDirFixture: testutils.NewTempDirFixture(t),
		t:              t,
	}
}

// pass `expectedRelative` = "" to indicate that `file` is NOT a child of `dir`
func (f *OspathFixture) assertChild(dir, file, expectedRel string) {
	rel, isChild := Child(dir, file)
	if expectedRel == "" {
		if isChild {
			f.t.Fatalf("Expected file '%s' to NOT be a child of dir '%s'", file, dir)
		}
		return
	}

	if !isChild {
		f.t.Fatalf("Expected file '%s' to be a child of dir '%s', but got !isChild", file, dir)
	}

	if rel != expectedRel {
		f.t.Fatalf("Expected relative path of '%s' to dir '%s' to be: '%s'. Actual: '%s'.", file, dir, expectedRel, rel)
	}
}
