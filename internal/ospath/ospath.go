package ospath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// filepath.Abs for testing
func MustAbs(path string) string {
	result, err := filepath.Abs(path)
	if err != nil {
		panic(fmt.Errorf("Abs(%s): %v", path, err))
	}
	return result
}

// Given absolute paths `dir` and `file`, returns
// the relative path of `file` relative to `dir`.
//
// Returns true if successful. If `file` is not under `dir`, returns false.
//
// TODO(nick): Should we have an assertion that dir and file are absolute? This is a common
// mistake when using this API. It won't work correctly with relative paths.
func Child(dir string, file string) (string, bool) {
	if dir == "" {
		return "", false
	}

	dir = filepath.Clean(dir)
	current := filepath.Clean(file)
	child := "."
	for {
		if strings.EqualFold(dir, current) {
			// If the two paths are exactly equal, then they must be the same.
			if dir == current {
				return child, true
			}

			// If the two paths are equal under case-folding, but not exactly equal,
			// then the only way to check if they're truly "equal" is to check
			// to see if we're on a case-insensitive file system.
			//
			// This is a notoriously tricky problem. See how dep solves it here:
			// https://github.com/golang/dep/blob/v0.5.4/internal/fs/fs.go#L33
			//
			// because you can mount case-sensitive filesystems onto case-insensitive
			// file-systems, and vice versa :scream:
			//
			// We want to do as much of this check as possible with strings-only
			// (to avoid a file system read and error handling), so we only
			// do this check if we have no other choice.
			dirInfo, err := os.Stat(dir)
			if err != nil {
				return "", false
			}

			currentInfo, err := os.Stat(current)
			if err != nil {
				return "", false
			}

			if !os.SameFile(dirInfo, currentInfo) {
				return "", false
			}
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
}

// IsChild returns true if the given file is a child of the given directory
func IsChild(dir string, file string) bool {
	_, ret := Child(dir, file)
	return ret
}

// IsChildOfOne returns true if the given file is a child of (at least) one of
// the given directories.
func IsChildOfOne(dirs []string, file string) bool {
	for _, dir := range dirs {
		if IsChild(dir, file) {
			return true
		}
	}
	return false
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

func IsDirLstat(path string) bool {
	f, err := os.Lstat(path)
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
