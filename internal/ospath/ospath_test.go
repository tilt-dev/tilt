package ospath

import (
	"os"
	"path"
	"testing"

	"github.com/windmilleng/tilt/internal/testutils/tempdir"
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

	f.assertChild("parent", "parent", ".")
}

func TestIsBrokenSymlink(t *testing.T) {
	f := NewOspathFixture(t)
	defer f.TearDown()

	f.TouchFiles([]string{
		"fileA",
		"child/fileB",
		"child/grandchild/fileC",
	})

	f.symlink("fileA", "symlinkFileA")
	f.symlink("fileB", "symlinkFileB")

	f.assertBrokenSymlink("fileA", false)
	f.assertBrokenSymlink("fileB", false)
	f.assertBrokenSymlink("child/fileB", false)
	f.assertBrokenSymlink("child/grandchild/fileC", false)
	f.assertBrokenSymlink("symlinkFileA", false)
	f.assertBrokenSymlink("symlinkFileB", true)
}

func TestInvalidDir(t *testing.T) {
	f := NewOspathFixture(t)
	defer f.TearDown()

	// Passing "" as dir used to make Child hang forever. Let's make sure it doesn't do that.
	f.assertChild("", "random", "")
}

func TestTryAsCwdChildren(t *testing.T) {
	f := NewOspathFixture(t)
	defer f.TearDown()
	oldPWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(oldPWD)
	os.Chdir(f.Path())

	results := TryAsCwdChildren([]string{f.Path()})

	if len(results) == 0 {
		t.Fatal("Expected 1 result, got 0")
	}

	r := results[0]

	if r != "." {
		t.Errorf("Expected %s to equal \".\"", r)
	}
}

type OspathFixture struct {
	*tempdir.TempDirFixture
	t *testing.T
}

func NewOspathFixture(t *testing.T) *OspathFixture {
	return &OspathFixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
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

func (f *OspathFixture) symlink(oldPath, newPath string) {
	oldPath = path.Join(f.Path(), oldPath)
	newPath = path.Join(f.Path(), newPath)
	err := os.Symlink(oldPath, newPath)
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *OspathFixture) assertBrokenSymlink(file string, expected bool) {
	broken, err := IsBrokenSymlink(path.Join(f.Path(), file))
	if err != nil {
		f.t.Fatal(err)
	}

	if broken != expected {
		if broken {
			f.t.Fatalf("Expected a regular file or working symlink: %s", file)
		} else {
			f.t.Fatalf("Expected a broken symlink: %s", file)
		}
	}
}
