package ignore

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/windmilleng/tilt/internal/dockerignore"
	"github.com/windmilleng/tilt/internal/git"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
)

type fileChangeFilter struct {
	ignoreMatchers model.PathMatcher
}

func (fcf fileChangeFilter) Matches(f string, isDir bool) (bool, error) {
	return fcf.ignoreMatchers.Matches(f, isDir)
}

type repoManifest interface {
	LocalRepos() []model.LocalGithubRepo
	Dockerignores() []model.Dockerignore
	TiltFilename() string
}

// Filter out files that should not be included in the build context.
func CreateBuildContextFilter(m repoManifest) model.PathMatcher {
	matchers := []model.PathMatcher{}
	if m.TiltFilename() != "" {
		m, err := model.NewSimpleFileMatcher(m.TiltFilename())
		if err == nil {
			matchers = append(matchers, m)
		}
	}
	for _, r := range m.LocalRepos() {
		gim, err := git.NewRepoIgnoreTester(context.Background(), r.LocalPath, r.GitignoreContents)
		if err == nil {
			matchers = append(matchers, gim)
		}
	}
	for _, r := range m.Dockerignores() {
		dim, err := dockerignore.DockerIgnoreTesterFromContents(r.LocalPath, r.Contents)
		if err == nil {
			matchers = append(matchers, dim)
		}
	}

	return model.NewCompositeMatcher(matchers)
}

type IgnorableManifest interface {
	LocalRepos() []model.LocalGithubRepo
	Dockerignores() []model.Dockerignore
}

// Filter out files that should not trigger new builds.
func CreateFileChangeFilter(m IgnorableManifest) (model.PathMatcher, error) {
	matchers := []model.PathMatcher{}
	for _, r := range m.LocalRepos() {
		gim, err := git.NewRepoIgnoreTester(context.Background(), r.LocalPath, r.GitignoreContents)
		if err == nil {
			matchers = append(matchers, gim)
		}
	}
	for _, di := range m.Dockerignores() {
		dim, err := dockerignore.DockerIgnoreTesterFromContents(di.LocalPath, di.Contents)
		if err == nil {
			matchers = append(matchers, dim)
		}
	}

	// Filter out spurious changes that we don't want to rebuild on, like IDE
	// temp/lock files.
	//
	// This isn't an ideal solution. In an ideal world, the user would put
	// everything to ignore in their gitignore/dockerignore files. This is a
	// stop-gap so they don't have a terrible experience if those files aren't
	// there or aren't in the right places.
	//
	// https://app.clubhouse.io/windmill/story/691/filter-out-ephemeral-file-changes
	matchers = append(matchers,
		// GoLand
		model.NewGlobMatcher("*___jb_old___", "*___jb_tmp___"),
		// Emacs
		tempBrokenSymlinkMatcher{},
	)

	ignoreMatcher := model.NewCompositeMatcher(matchers)

	// TODO(maia): this doesn't have to be a composite matcher anymore since removing `configMatcher`?
	return fileChangeFilter{
		ignoreMatchers: ignoreMatcher,
	}, nil
}

func CreateStepMatcher(s model.Step) (model.PathMatcher, error) {
	dim, err := dockerignore.NewDockerPatternMatcher(s.BaseDirectory, s.Triggers)
	if err != nil {
		return nil, err
	}

	return dim, nil
}

// Emacs temp files look like:
// .#a.txt -> [some garbage]
type tempBrokenSymlinkMatcher struct{}

func (m tempBrokenSymlinkMatcher) Matches(path string, isDir bool) (bool, error) {
	if isDir {
		return false, nil
	}

	if !strings.HasPrefix(filepath.Base(path), ".") {
		return false, nil
	}

	return ospath.IsBrokenSymlink(path)
}
