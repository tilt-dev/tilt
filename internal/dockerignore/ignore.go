package dockerignore

import (
	"os"
	"path/filepath"

	"github.com/windmilleng/tilt/internal/ignore"

	"github.com/docker/docker/builder/dockerignore"
	"github.com/docker/docker/pkg/fileutils"
)

type dockerfileIgnoreTester struct {
	repoRoot string
	matcher  *fileutils.PatternMatcher
}

var _ ignore.Tester = dockerfileIgnoreTester{}

func (i dockerfileIgnoreTester) IsIgnored(f string, isDir bool) (bool, error) {
	rp, err := filepath.Rel(i.repoRoot, f)
	if err != nil {
		return false, err
	}

	return i.matcher.Matches(rp)
}

func NewDockerfileIgnoreTester(repoRoot string) (ignore.Tester, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}

	patterns, err := readDockerignorePatterns(absRoot)
	if err != nil {
		return nil, err
	}

	pm, err := fileutils.NewPatternMatcher(patterns)
	if err != nil {
		return nil, err
	}

	return dockerfileIgnoreTester{
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

func NewMultiRepoDockerfileIgnoreTester(repoRoots []string) (ignore.Tester, error) {
	var testers []ignore.Tester
	for _, repoRoot := range repoRoots {
		t, err := NewDockerfileIgnoreTester(repoRoot)
		if err != nil {
			return nil, err
		}
		testers = append(testers, t)
	}

	return ignore.CompositeIgnoreTester{Testers: testers}, nil
}
