package ospath

import (
	"os"
	"path/filepath"

	"github.com/windmilleng/tesseract/pkg/pathutil"
)

func Child(dir string, file string) (string, bool) {
	return pathutil.Child(theOsPathUtil, dir, file)
}

func RealChild(dir string, file string) (string, bool, error) {
	realDir, err := RealAbs(dir)
	if err != nil {
		return "", false, err
	}
	realFile, err := RealAbs(file)
	if err != nil {
		return "", false, err
	}

	rel, isChild := Child(realDir, realFile)
	return rel, isChild, nil
}

// Returns the absolute version of this path, resolving all symlinks.
func RealAbs(path string) (string, error) {
	// Make the path absolute first, so that we find any symlink parents.
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}

	// Resolve the symlinks.
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return "", err
	}

	// Double-check we're still absolute.
	return filepath.Abs(realPath)
}

// Like os.Getwd, but with all symlinks resolved.
func Realwd() (string, error) {
	path, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return RealAbs(path)
}

func IsDir(path string) bool {
	f, err := os.Stat(path)
	if err != nil {
		return false
	}

	return f.Mode().IsDir()
}

type osPathUtil struct{}

func (p osPathUtil) Base(path string) string                  { return filepath.Base(path) }
func (p osPathUtil) Dir(path string) string                   { return filepath.Dir(path) }
func (p osPathUtil) Join(a, b string) string                  { return filepath.Join(a, b) }
func (p osPathUtil) Match(pattern, path string) (bool, error) { return filepath.Match(pattern, path) }
func (p osPathUtil) Separator() rune                          { return filepath.Separator }

var theOsPathUtil = osPathUtil{}
