package dockerignore

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/builder/dockerignore"
	"github.com/docker/docker/pkg/fileutils"
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

func (i dockerPathMatcher) AsMatchPatterns() []string {
	result := []string{}
	for _, p := range i.matcher.Patterns() {
		result = append(result, p.String())
	}
	return result
}

func (i dockerPathMatcher) MatchesEntireDir(f string) (bool, error) {
	matches, err := i.Matches(f)
	if !matches || err != nil {
		return matches, err
	}

	// We match the dir, but we might exclude files underneath it.
	if i.matcher.Exclusions() {
		// TODO(nick): Add more complex logic for interpreting exclusion patterns.
		return false, nil
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
