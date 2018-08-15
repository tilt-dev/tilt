// Common utilities for DB paths and OS paths

package pathutil

import (
	"strings"
)

type PathUtil interface {
	Base(path string) string
	Dir(path string) string
	Join(dir, base string) string
	Match(pattern, name string) (matched bool, err error)
	Separator() rune
}

// Split the path into (firstDir, rest). Note that this is
// different from normal Split(), which splits things into (dir, base).
// SplitFirst is better for matching algorithms.
//
// If the path cannot be split, returns (p, "").
func SplitFirst(util PathUtil, p string) (string, string) {
	firstSlash := strings.IndexRune(p, util.Separator())
	if firstSlash == -1 {
		return p, ""
	}

	return p[0:firstSlash], p[firstSlash+1:]
}

// Given absolute paths `dir` and `file`, returns
// the relative path of `file` relative to `dir`.
//
// Returns true if successful. If `file` is not under `dir`, returns false.
func Child(util PathUtil, dir, file string) (string, bool) {
	current := file
	child := ""
	for true {
		if dir == current {
			return child, true
		}

		if len(current) <= len(dir) || current == "." {
			return "", false
		}

		cDir := util.Dir(current)
		cBase := util.Base(current)
		child = util.Join(cBase, child)
		current = cDir
	}

	return "", false
}
