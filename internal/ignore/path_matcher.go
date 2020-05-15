package ignore

import (
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/dockerignore"
	"github.com/tilt-dev/tilt/internal/git"
	"github.com/tilt-dev/tilt/internal/ospath"
	"github.com/tilt-dev/tilt/pkg/model"
)

type repoTarget interface {
	LocalRepos() []model.LocalGitRepo
	Dockerignores() []model.Dockerignore
	TiltFilename() string
}

// Filter out files that should not be included in the build context.
func CreateBuildContextFilter(m repoTarget) model.PathMatcher {
	matchers := []model.PathMatcher{}
	if m.TiltFilename() != "" {
		m, err := model.NewSimpleFileMatcher(m.TiltFilename())
		if err == nil {
			matchers = append(matchers, m)
		}
	}
	for _, r := range m.LocalRepos() {
		matchers = append(matchers, git.NewRepoIgnoreTester(r.LocalPath))
	}
	for _, r := range m.Dockerignores() {
		dim, err := dockerignore.DockerIgnoreTesterFromContents(r.LocalPath, r.Contents)
		if err == nil {
			matchers = append(matchers, dim)
		}
	}

	return model.NewCompositeMatcher(matchers)
}

type IgnorableTarget interface {
	LocalRepos() []model.LocalGitRepo
	Dockerignores() []model.Dockerignore

	// These directories and their children will not trigger file change events
	IgnoredLocalDirectories() []string
}

// Filter out files that should not trigger new builds.
func CreateFileChangeFilter(m IgnorableTarget) (model.PathMatcher, error) {
	matchers := []model.PathMatcher{}
	for _, r := range m.LocalRepos() {
		matchers = append(matchers, git.NewRepoIgnoreTester(r.LocalPath))
	}
	for _, di := range m.Dockerignores() {
		dim, err := dockerignore.DockerIgnoreTesterFromContents(di.LocalPath, di.Contents)
		if err == nil {
			matchers = append(matchers, dim)
		}
	}
	for _, p := range m.IgnoredLocalDirectories() {
		dm, err := newDirectoryMatcher(p)
		if err != nil {
			return nil, errors.Wrap(err, "creating directory matcher")
		}
		matchers = append(matchers, dm)
	}

	matchers = append(matchers, ephemeralPathMatcher)

	return model.NewCompositeMatcher(matchers), nil
}

func CreateRunMatcher(r model.Run) (model.PathMatcher, error) {
	dim, err := dockerignore.NewDockerPatternMatcher(r.Triggers.BaseDirectory, r.Triggers.Paths)
	if err != nil {
		return nil, err
	}

	return dim, nil
}

type directoryMatcher struct {
	dir string
}

var _ model.PathMatcher = directoryMatcher{}

func newDirectoryMatcher(dir string) (directoryMatcher, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return directoryMatcher{}, errors.Wrapf(err, "failed to get abs path of '%s'", dir)
	}
	return directoryMatcher{dir}, nil
}

func (d directoryMatcher) Matches(p string) (bool, error) {
	return ospath.IsChild(d.dir, p), nil
}

func (d directoryMatcher) MatchesEntireDir(p string) (bool, error) {
	return d.Matches(p)
}
