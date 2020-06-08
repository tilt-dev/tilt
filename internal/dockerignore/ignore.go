package dockerignore

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/builder/dockerignore"
	"github.com/docker/docker/pkg/fileutils"

	"github.com/tilt-dev/tilt/internal/ospath"
)

type dockerPathMatcher struct {
	repoRoot string
	matcher  *fileutils.PatternMatcher
}

func (i dockerPathMatcher) Matches(f string) (bool, error) {
	rp, err := filepath.Rel(i.repoRoot, f)
	if err != nil {
		return false, err
	}
	return i.matcher.Matches(rp)
}

func (i dockerPathMatcher) MatchesEntireDir(f string) (bool, error) {
	matches, err := i.Matches(f)
	if !matches || err != nil {
		return matches, err
	}

	// We match the dir, but we might exclude files underneath it.
	if i.matcher.Exclusions() {
		for _, pattern := range i.matcher.Patterns() {
			if !pattern.Exclusion() {
				continue
			}
			absPattern := filepath.Join(i.repoRoot, pattern.String())
			if ospath.IsChild(f, absPattern) {
				// Found an exclusion match -- we don't match this whole dir
				return false, nil
			}
		}
		return true, nil
	}
	return true, nil
}

func NewDockerIgnoreTester(repoRoot string) (*dockerPathMatcher, error) {
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

func NewDockerPatternMatcher(repoRoot string, patterns []string) (*dockerPathMatcher, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}

	pm, err := fileutils.NewPatternMatcher(patterns)
	if err != nil {
		return nil, err
	}

	return &dockerPathMatcher{
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

func DockerIgnoreTesterFromContents(repoRoot string, contents string) (*dockerPathMatcher, error) {
	patterns, err := dockerignore.ReadAll(strings.NewReader(contents))
	if err != nil {
		return nil, err
	}

	return NewDockerPatternMatcher(repoRoot, patterns)
}
