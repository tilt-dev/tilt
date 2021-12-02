package ignore

import (
	"fmt"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/dockerignore"
	"github.com/tilt-dev/tilt/internal/git"
	"github.com/tilt-dev/tilt/internal/ospath"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
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
		dim, err := dockerignore.NewDockerPatternMatcher(r.LocalPath, r.Patterns)
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

// Interpret the FileWatch Ignores as a path matcher.
func IgnoresToMatcher(ignores []v1alpha1.IgnoreDef) (model.PathMatcher, error) {
	var ignoreMatchers []model.PathMatcher
	for _, ignoreDef := range ignores {
		if len(ignoreDef.Patterns) != 0 {
			m, err := dockerignore.NewDockerPatternMatcher(
				ignoreDef.BasePath,
				append([]string{}, ignoreDef.Patterns...))
			if err != nil {
				return nil, fmt.Errorf("invalid ignore def: %v", err)
			}
			ignoreMatchers = append(ignoreMatchers, m)
		} else {
			m, err := NewDirectoryMatcher(ignoreDef.BasePath)
			if err != nil {
				return nil, fmt.Errorf("invalid ignore def: %v", err)
			}
			ignoreMatchers = append(ignoreMatchers, m)
		}
	}
	// ephemeral OS/IDE stuff is not part of the spec but always included
	ignoreMatchers = append(ignoreMatchers, EphemeralPathMatcher)

	return model.NewCompositeMatcher(ignoreMatchers), nil
}

// Pull the FileWatch Ignores out of the old manifest target data model.
func TargetToFileWatchIgnores(t IgnorableTarget) (ignores []v1alpha1.IgnoreDef) {
	if iTarget, ok := t.(model.ImageTarget); ok && iTarget.TiltFilename() != "" {
		ignores = append(ignores, v1alpha1.IgnoreDef{BasePath: iTarget.TiltFilename()})
	}

	for _, r := range t.LocalRepos() {
		ignores = append(ignores, v1alpha1.IgnoreDef{
			BasePath: filepath.Join(r.LocalPath, ".git"),
		})
	}

	for _, di := range t.Dockerignores() {
		if di.Empty() {
			continue
		}
		ignores = append(ignores, v1alpha1.IgnoreDef{
			BasePath: di.LocalPath,
			Patterns: append([]string(nil), di.Patterns...),
		})
	}
	for _, ild := range t.IgnoredLocalDirectories() {
		ignores = append(ignores, v1alpha1.IgnoreDef{
			BasePath: ild,
		})
	}
	return ignores
}

// Filter out files that should not trigger new builds.
func CreateFileChangeFilter(m IgnorableTarget) (model.PathMatcher, error) {
	return IgnoresToMatcher(TargetToFileWatchIgnores(m))
}

type DirectoryMatcher struct {
	dir string
}

var _ model.PathMatcher = DirectoryMatcher{}

func NewDirectoryMatcher(dir string) (DirectoryMatcher, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return DirectoryMatcher{}, errors.Wrapf(err, "failed to get abs path of '%s'", dir)
	}
	return DirectoryMatcher{dir}, nil
}

func (d DirectoryMatcher) Dir() string {
	return d.dir
}

func (d DirectoryMatcher) Matches(p string) (bool, error) {
	return ospath.IsChild(d.dir, p), nil
}

func (d DirectoryMatcher) MatchesEntireDir(p string) (bool, error) {
	return d.Matches(p)
}
