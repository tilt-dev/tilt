package model

import (
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/ospath"
)

type PathMatcher interface {
	Matches(f string) (bool, error)

	// If this matches the entire dir, we can often optimize filetree walks a bit
	MatchesEntireDir(file string) (bool, error)
}

// A Matcher that matches nothing.
type emptyMatcher struct{}

func (m emptyMatcher) Matches(f string) (bool, error) {
	return false, nil
}
func (emptyMatcher) MatchesEntireDir(p string) (bool, error) { return false, nil }

var EmptyMatcher PathMatcher = emptyMatcher{}

// A matcher that matches exactly against a set of files.
type fileMatcher struct {
	paths map[string]bool
}

func (m fileMatcher) Matches(f string) (bool, error) {
	return m.paths[f], nil
}
func (fileMatcher) MatchesEntireDir(f string) (bool, error) { return false, nil }

// NewSimpleFileMatcher returns a matcher for the given paths; any relative paths
// are converted to absolute (relative to cwd).
func NewSimpleFileMatcher(paths ...string) (fileMatcher, error) {
	pathMap := make(map[string]bool, len(paths))
	for _, path := range paths {
		// Get the absolute path of the path, because PathMatchers expect to always
		// work with absolute paths.
		path, err := filepath.Abs(path)
		if err != nil {
			return fileMatcher{}, errors.Wrap(err, "NewSimplePathMatcher")
		}
		pathMap[path] = true
	}
	return fileMatcher{paths: pathMap}, nil
}

// This matcher will match a path if it is:
// A. an exact match for one of matcher.paths, or
// B. the child of a path in matcher.paths
// e.g. if paths = {"foo.bar", "baz/"}, will match both
// A. "foo.bar" (exact match), and
// B. "baz/qux" (child of one of the paths)
type fileOrChildMatcher struct {
	paths map[string]bool
}

func (m fileOrChildMatcher) Matches(f string) (bool, error) {
	// (A) Exact match
	if m.paths[f] {
		return true, nil
	}

	// (B) f is child of any of m.paths
	for path := range m.paths {
		if ospath.IsChild(path, f) {
			return true, nil
		}
	}

	return false, nil
}

func (m fileOrChildMatcher) MatchesEntireDir(f string) (bool, error) {
	return m.Matches(f)
}

// NewRelativeFileOrChildMatcher returns a matcher for the given paths (with any
// relative paths converted to absolute, relative to the given baseDir).
func NewRelativeFileOrChildMatcher(baseDir string, paths ...string) fileOrChildMatcher {
	pathMap := make(map[string]bool, len(paths))
	for _, path := range paths {
		if !filepath.IsAbs(path) {
			path = filepath.Join(baseDir, path)
		}
		pathMap[path] = true
	}
	return fileOrChildMatcher{paths: pathMap}
}

// A PathSet stores one or more filepaths, along with the directory that any
// relative paths are relative to
// NOTE(maia): in its current usage (for LiveUpdate.Run.Triggers, LiveUpdate.FallBackOnFiles())
// this isn't strictly necessary, could just as easily convert paths to Abs when specified in
// the Tiltfile--but leaving this code in place for now because it was already written and
// may help with complicated future cases (glob support, etc.)
type PathSet struct {
	Paths         []string
	BaseDirectory string
}

func NewPathSet(paths []string, baseDir string) PathSet {
	return PathSet{
		Paths:         paths,
		BaseDirectory: baseDir,
	}
}

func (ps PathSet) Empty() bool { return len(ps.Paths) == 0 }

// AnyMatch returns true if any of the given filepaths match any paths contained in the pathset
// (along with the first path that matched).
func (ps PathSet) AnyMatch(paths []string) (bool, string, error) {
	matcher := NewRelativeFileOrChildMatcher(ps.BaseDirectory, ps.Paths...)

	for _, path := range paths {
		match, err := matcher.Matches(path)
		if err != nil {
			return false, "", err
		}
		if match {
			return true, path, nil
		}
	}
	return false, "", nil
}

type CompositePathMatcher struct {
	Matchers []PathMatcher
}

func NewCompositeMatcher(matchers []PathMatcher) PathMatcher {
	if len(matchers) == 0 {
		return EmptyMatcher
	}
	return CompositePathMatcher{Matchers: matchers}
}

func (c CompositePathMatcher) Matches(f string) (bool, error) {
	for _, t := range c.Matchers {
		ret, err := t.Matches(f)
		if err != nil {
			return false, err
		}
		if ret {
			return true, nil
		}
	}
	return false, nil
}

func (c CompositePathMatcher) MatchesEntireDir(f string) (bool, error) {
	for _, t := range c.Matchers {
		matches, err := t.MatchesEntireDir(f)
		if matches || err != nil {
			return matches, err
		}
	}
	return false, nil
}

var _ PathMatcher = CompositePathMatcher{}
