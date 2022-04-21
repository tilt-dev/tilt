package ignore

import (
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/tilt-dev/tilt/internal/dockerignore"
	"github.com/tilt-dev/tilt/internal/ospath"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// Filter out files that should not be included in the build context.
func CreateBuildContextFilter(ignores []v1alpha1.IgnoreDef) model.PathMatcher {
	return model.NewCompositeMatcher(ToMatchersBestEffort(ignores))
}

// Filter out files that should not trigger new builds.
func CreateFileChangeFilter(ignores []v1alpha1.IgnoreDef) model.PathMatcher {
	return model.NewCompositeMatcher(
		append(ToMatchersBestEffort(ignores), EphemeralPathMatcher))
}

// Interpret ignores as a PathMatcher, skipping ignores that are ill-formed.
func ToMatchersBestEffort(ignores []v1alpha1.IgnoreDef) []model.PathMatcher {
	var ignoreMatchers []model.PathMatcher
	for _, ignoreDef := range ignores {
		if len(ignoreDef.Patterns) != 0 {
			m, err := dockerignore.NewDockerPatternMatcher(
				ignoreDef.BasePath,
				append([]string{}, ignoreDef.Patterns...))
			if err == nil {
				ignoreMatchers = append(ignoreMatchers, m)
			}
		} else {
			m, err := NewDirectoryMatcher(ignoreDef.BasePath)
			if err == nil {
				ignoreMatchers = append(ignoreMatchers, m)
			}
		}
	}
	return ignoreMatchers
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
