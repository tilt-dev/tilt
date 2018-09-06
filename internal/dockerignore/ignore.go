package dockerignore

import (
	"path"
	"path/filepath"

	"github.com/codeskyblue/dockerignore"
	"github.com/windmilleng/tilt/internal/ignore"
)

type dockerfileIgnoreTester struct {
	repoRoot string
	patterns []string
}

var _ ignore.Tester = dockerfileIgnoreTester{}

func (i dockerfileIgnoreTester) IsIgnored(f string, isDir bool) (bool, error) {
	isSkip, err := dockerignore.Matches(f, i.patterns)
	if err != nil {
		return false, err
	}

	return isSkip, nil
}

func NewDockerfileIgnoreTester(repoRoot string) (ignore.Tester, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}

	p := path.Join(absRoot, ".dockerignore")
	patterns, err := dockerignore.ReadIgnoreFile(p)
	if err != nil {
		return dockerfileIgnoreTester{}, err
	}

	rp := []string{}
	for _, p := range patterns {
		rp = append(rp, filepath.Join(absRoot, p))
	}

	return dockerfileIgnoreTester{
		repoRoot: absRoot,
		patterns: rp,
	}, nil
}
