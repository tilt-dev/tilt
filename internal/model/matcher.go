package model

import (
	"path/filepath"
)

type PathMatcher interface {
	Matches(f string, isDir bool) (bool, error)
}

type CompositePathMatcher struct {
	Matchers []PathMatcher
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

var _ PathMatcher = CompositePathMatcher{}

type singlePathMatcher struct {
	repoRoot string
	path     string
}

func (p singlePathMatcher) Matches(f string, isDir bool) (bool, error) {
	rp, err := filepath.Rel(p.repoRoot, f)
	if err != nil {
		return false, err
	}

	return rp == p.path, nil
}

func NewPathMatcher(repoRoot string, path string) (PathMatcher, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}

	return singlePathMatcher{
		repoRoot: absRoot,
		path:     path,
	}, nil
}
