package ignore

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

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
		// We actually don't want to filter out the contents of .gitignore in a build. There are lots of situation where
		// a directory is .gitignored (vendor) but does need to be included in the build context.
		// As a result leave the contents as the empty string here so we ignore the gitiginore contents but
		// do ignore the `.git` directory.
		gim, err := git.NewRepoIgnoreTester(context.Background(), r.LocalPath, "")
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
	for _, p := range m.IgnoredLocalDirectories() {
		dm, err := newDirectoryMatcher(p)
		if err != nil {
			return nil, errors.Wrap(err, "creating directory matcher")
		}
		matchers = append(matchers, dm)
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

func (d directoryMatcher) Matches(p string, isDir bool) (bool, error) {
	_, isChild := ospath.Child(d.dir, p)
	return isChild, nil
}
