package dockerignore

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/builder/dockerignore"
	tiltDockerignore "github.com/tilt-dev/dockerignore"
	"github.com/yookoala/realpath"

	"github.com/tilt-dev/tilt/internal/ospath"
)

type dockerPathMatcher struct {
	repoRoot string
	matcher  *tiltDockerignore.PatternMatcher
}

func (i dockerPathMatcher) Matches(f string) (bool, error) {
	if !filepath.IsAbs(f) {
		f = filepath.Join(i.repoRoot, f)
	}
	return i.matcher.Matches(f)
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
			if ospath.IsChild(f, pattern.String()) {
				// Found an exclusion match -- we don't match this whole dir
				return false, nil
			}
		}
		return true, nil
	}
	return true, nil
}

func NewDockerIgnoreTester(repoRoot string) (*dockerPathMatcher, error) {
	absRoot, err := ospath.RealAbs(repoRoot)
	if err != nil {
		return nil, err
	}

	patterns, err := readDockerignorePatterns(absRoot)
	if err != nil {
		return nil, err
	}

	return NewDockerPatternMatcher(absRoot, patterns)
}

// Make all the patterns use absolute paths.
func absPatterns(absRoot string, patterns []string) []string {
	absPatterns := make([]string, 0, len(patterns))
	for _, p := range patterns {
		// The pattern parsing here is loosely adapted from fileutils' NewPatternMatcher
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		p = filepath.Clean(p)

		pPath := p
		isExclusion := false
		if p[0] == '!' {
			pPath = p[1:]
			isExclusion = true
		}

		if !filepath.IsAbs(pPath) {
			pPath = filepath.Join(absRoot, pPath)
		}
		absPattern := pPath
		if isExclusion {
			absPattern = fmt.Sprintf("!%s", pPath)
		}
		absPatterns = append(absPatterns, absPattern)
	}
	return absPatterns
}

func NewDockerPatternMatcher(repoRoot string, patterns []string) (*dockerPathMatcher, error) {
	absRoot, err := ospath.RealAbs(repoRoot)
	realAbsRoot, errDos := realpath.Realpath(repoRoot)

	fmt.Println("docker abspath", absRoot)
	fmt.Println("docker abspath from package:", realAbsRoot)
	fmt.Println("are they the same?", absRoot == realAbsRoot)

	if err != nil || errDos != nil {
		return nil, err
	}

	pm, err := tiltDockerignore.NewPatternMatcher(absPatterns(absRoot, patterns))
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
