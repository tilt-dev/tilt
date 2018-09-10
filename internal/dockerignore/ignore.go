package dockerignore

import (
	"os"
	"path/filepath"

	"github.com/windmilleng/tilt/internal/model"

	"github.com/docker/docker/builder/dockerignore"
	"github.com/docker/docker/pkg/fileutils"
)

type dockerPathMatcher struct {
	repoRoot string
	matcher  *fileutils.PatternMatcher
}

var _ model.PathMatcher = dockerPathMatcher{}

func (i dockerPathMatcher) Matches(f string, isDir bool) (bool, error) {
	rp, err := filepath.Rel(i.repoRoot, f)
	if err != nil {
		return false, err
	}

	return i.matcher.Matches(rp)
}

func NewDockerIgnoreTester(repoRoot string) (model.PathMatcher, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}

	patterns, err := readDockerignorePatterns(absRoot)
	if err != nil {
		return nil, err
	}

	return NewDockerPatternMatcher(absRoot, patterns)
}

func NewDockerPatternMatcher(repoRoot string, patterns []string) (model.PathMatcher, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}

	pm, err := fileutils.NewPatternMatcher(patterns)
	if err != nil {
		return nil, err
	}

	return dockerPathMatcher{
		repoRoot: absRoot,
		matcher:  pm,
	}, nil
}

func readDockerignorePatterns(repoRoot string) ([]string, error) {
	var excludes []string

	f, err := os.Open(filepath.Join(repoRoot, ".dockerignore"))
	switch {
	case os.IsNotExist(err):
		return excludes, nil
	case err != nil:
		return nil, err
	}
	defer func() { _ = f.Close() }()

	return dockerignore.ReadAll(f)
}

func NewMultiRepoDockerIgnoreTester(repoRoots []string) (model.PathMatcher, error) {
	var testers []model.PathMatcher
	for _, repoRoot := range repoRoots {
		t, err := NewDockerIgnoreTester(repoRoot)
		if err != nil {
			return nil, err
		}
		testers = append(testers, t)
	}

	return model.CompositePathMatcher{Matchers: testers}, nil
}
