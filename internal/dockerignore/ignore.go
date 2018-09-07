package dockerignore

import (
	"os"
	"path"
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

	p := path.Join(absRoot, ".dockerignore")
	var patterns []string

	f, err := os.Open(p)
	defer func() { _ = f.Close() }()
	switch {
	case os.IsNotExist(err):
		pm, err := fileutils.NewPatternMatcher(patterns)
		if err != nil {
			return nil, err
		}

		return dockerfileIgnoreTester{
			repoRoot: absRoot,
			matcher:  pm,
		}, err
	case err != nil:
		return nil, err
	}

	patterns, err = dockerignore.ReadAll(f)
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
