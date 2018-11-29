package model

import (
	"path/filepath"

	"github.com/gobwas/glob"
	"github.com/pkg/errors"
)

type PathMatcher interface {
	Matches(f string, isDir bool) (bool, error)
}

// A Matcher that matches nothing.
type emptyMatcher struct{}

func (m emptyMatcher) Matches(f string, isDir bool) (bool, error) {
	return false, nil
}

var EmptyMatcher PathMatcher = emptyMatcher{}

// A matcher that matches against a set of files.
type fileMatcher struct {
	paths map[string]bool
}

func (m fileMatcher) Matches(f string, isDir bool) (bool, error) {
	return m.paths[f], nil
}

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

type globMatcher struct {
	globs []glob.Glob
}

func (gm globMatcher) Matches(f string, isDir bool) (bool, error) {
	for _, g := range gm.globs {
		if g.Match(f) {
			return true, nil
		}
	}

	return false, nil
}

func NewGlobMatcher(globs ...string) PathMatcher {
	ret := globMatcher{}
	for _, g := range globs {
		ret.globs = append(ret.globs, glob.MustCompile(g))
	}

	return ret
}

type PatternMatcher interface {
	PathMatcher

	// Express this PathMatcher as a sequence of filepath.Match
	// patterns. These patterns are widely useful in Docker-land because
	// they're suitable in .dockerignore or Dockerfile ADD statements
	// https://docs.docker.com/engine/reference/builder/#add
	AsMatchPatterns() []string
}

type CompositePathMatcher struct {
	Matchers []PathMatcher
}

func NewCompositeMatcher(matchers []PathMatcher) PathMatcher {
	if len(matchers) == 0 {
		return EmptyMatcher
	}
	cMatcher := CompositePathMatcher{Matchers: matchers}
	pMatchers := make([]PatternMatcher, len(matchers))
	for i, m := range matchers {
		pm, ok := m.(CompositePatternMatcher)
		if !ok {
			return cMatcher
		}
		pMatchers[i] = pm
	}
	return CompositePatternMatcher{
		CompositePathMatcher: cMatcher,
		Matchers:             pMatchers,
	}
}

func (c CompositePathMatcher) Matches(f string, isDir bool) (bool, error) {
	for _, t := range c.Matchers {
		ret, err := t.Matches(f, isDir)
		if err != nil {
			return false, err
		}
		if ret {
			return true, nil
		}
	}
	return false, nil
}

type CompositePatternMatcher struct {
	CompositePathMatcher
	Matchers []PatternMatcher
}

func (c CompositePatternMatcher) AsMatchPatterns() []string {
	result := []string{}
	for _, m := range c.Matchers {
		result = append(result, m.AsMatchPatterns()...)
	}
	return result
}

var _ PathMatcher = CompositePathMatcher{}
var _ PatternMatcher = CompositePatternMatcher{}
