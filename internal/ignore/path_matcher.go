package ignore

import (
	"context"

	"github.com/windmilleng/tilt/internal/dockerignore"
	"github.com/windmilleng/tilt/internal/git"
	"github.com/windmilleng/tilt/internal/model"
)

type fileChangeFilter struct {
	ignoreMatchers model.PathMatcher
	configMatcher  model.PathMatcher
}

func (fcf fileChangeFilter) Matches(f string, isDir bool) (bool, error) {
	configMatches, err := fcf.configMatcher.Matches(f, isDir)
	if configMatches && err == nil {
		return false, nil
	}

	return fcf.ignoreMatchers.Matches(f, isDir)
}

// Filter out files that should not be included in the build context.
func CreateBuildContextFilter(m model.Manifest) model.PathMatcher {
	matchers := []model.PathMatcher{}
	if m.TiltFilename != "" {
		m, err := model.NewSimpleFileMatcher(m.TiltFilename)
		if err == nil {
			matchers = append(matchers, m)
		}
	}
	for _, r := range m.Repos {
		gim, err := git.NewRepoIgnoreTester(context.Background(), r.LocalPath, r.GitignoreContents)
		if err == nil {
			matchers = append(matchers, gim)
		}

		dim, err := dockerignore.DockerIgnoreTesterFromContents(r.LocalPath, r.DockerignoreContents)
		if err == nil {
			matchers = append(matchers, dim)
		}
	}

	return model.NewCompositeMatcher(matchers)
}

// Filter out files that should not trigger new builds.
func CreateFileChangeFilter(m model.Manifest) model.PathMatcher {
	matchers := []model.PathMatcher{}
	configMatcher, err := m.ConfigMatcher()
	if err == nil {
		matchers = append(matchers, configMatcher)
	}

	for _, r := range m.Repos {
		gim, err := git.NewRepoIgnoreTester(context.Background(), r.LocalPath, r.GitignoreContents)
		if err == nil {
			matchers = append(matchers, gim)
		}

		dim, err := dockerignore.DockerIgnoreTesterFromContents(r.LocalPath, r.DockerignoreContents)
		if err == nil {
			matchers = append(matchers, dim)
		}
	}

	ignoreMatcher := model.NewCompositeMatcher(matchers)

	return fileChangeFilter{
		ignoreMatchers: ignoreMatcher,
		configMatcher:  configMatcher,
	}
}

func CreateStepMatcher(s model.Step) (model.PathMatcher, error) {
	dim, err := dockerignore.NewDockerPatternMatcher(s.BaseDirectory, s.Triggers)
	if err != nil {
		return nil, err
	}

	return dim, nil
}
