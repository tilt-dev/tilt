package ospath

import (
	"os"
	"path/filepath"
)

// Given absolute paths `dir` and `file`, returns
// the relative path of `file` relative to `dir`.
//
// Returns true if successful. If `file` is not under `dir`, returns false.
func Child(dir string, file string) (string, bool) {
	if dir == "" {
		return "", false
	}

	current := file
	child := "."
	for true {
		if dir == current {
			return child, true
		}

		if len(current) <= len(dir) || current == "." {
			return "", false
		}

		cDir := filepath.Dir(current)
		cBase := filepath.Base(current)
		child = filepath.Join(cBase, child)
		current = cDir
	}

	return "", false
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

func IsRegularFile(path string) bool {
	f, err := os.Stat(path)
	if err != nil {
		return false
	}

	return f.Mode().IsRegular()
}

func IsDir(path string) bool {
	f, err := os.Stat(path)
	if err != nil {
		return false
	}

	return f.Mode().IsDir()
}

func IsBrokenSymlink(path string) (bool, error) {
	// Stat resolves symlinks, lstat does not.
	// So if Stat reports IsNotExist, but Lstat does not,
	// then we have a broken symlink.
	_, err := os.Stat(path)
	if err == nil {
		return false, nil
	}

	if !os.IsNotExist(err) {
		return false, err
	}

	_, err = os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// TryAsCwdChildren converts the given absolute paths to children of the CWD,
// if possible (otherwise, leaves them as absolute paths).
func TryAsCwdChildren(absPaths []string) []string {
	wd, err := os.Getwd()
	if err != nil {
		// This is just a util for printing right now, so don't actually throw an
		// error, just return back all the absolute paths
		return absPaths[:]
	}

	res := make([]string, len(absPaths))
	for i, abs := range absPaths {
		rel, isChild := Child(wd, abs)
		if isChild {
			res[i] = rel
		} else {
			res[i] = abs
		}
	}
	return res
}
