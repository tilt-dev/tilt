package git

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/monochromegane/go-gitignore"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
)

// Known feature differences from git:
// 1. does not use git config core.excludesfile
// 2. only looks for .gitignore in repo root, instead of all directories between dirname(file) and repo root
// 3. does not use .git/info/exclude
// 4. does not take index into account

// an IgnoreTester that ignores nothing
type FalseIgnoreTester struct{}

func (FalseIgnoreTester) Matches(f string, isDir bool) (bool, error) {
	return false, nil
}

var _ model.PathMatcher = FalseIgnoreTester{}

// ignores files specified in .gitignore
type gitIgnoreTester struct {
	repoRoot      string
	ignoreMatcher gitignore.IgnoreMatcher
}

var _ model.PathMatcher = gitIgnoreTester{}

func (i gitIgnoreTester) Matches(f string, isDir bool) (bool, error) {
	_, isChild := ospath.Child(i.repoRoot, f)
	if !isChild {
		return false, nil
	}

	return i.ignoreMatcher.Match(f, isDir), nil
}

func NewGitIgnoreTester(ctx context.Context, repoRoot string) (model.PathMatcher, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}

	p := path.Join(absRoot, ".gitignore")
	i, err := gitignore.NewGitIgnore(p)
	if err != nil {
		_, err = os.Open(path.Join(absRoot, ".gitignore"))

		pathError, ok := err.(*os.PathError)
		//if the error is that file isn't there (ENOENT), then we don't need a warning, since that's a normal case
		//if it's any other error, log and pretend the file doesn't exist (matching git's behavior)
		if ok && pathError.Err != syscall.ENOENT {
			logger.Get(ctx).Verbosef("failed to open gitignore %v: %v", p, err)
		}
		return &FalseIgnoreTester{}, nil
	}
	return &gitIgnoreTester{absRoot, i}, nil
}

// ignores files specified in .gitignore plus any files in $ROOT/.git/
type repoIgnoreTester struct {
	repoRoot        string
	gitIgnoreTester model.PathMatcher
}

var _ model.PathMatcher = repoIgnoreTester{}

func (r repoIgnoreTester) Matches(f string, isDir bool) (bool, error) {
	// TODO(matt) what do we want to do with symlinks?
	absPath, err := filepath.Abs(f)
	if err != nil {
		return false, err
	}

	if strings.HasPrefix(absPath, filepath.Join(r.repoRoot, ".git/")) {
		return true, nil
	}

	return r.gitIgnoreTester.Matches(f, isDir)
}

func NewRepoIgnoreTester(ctx context.Context, repoRoot string) (model.PathMatcher, error) {
	g, err := NewGitIgnoreTester(ctx, repoRoot)
	if err != nil {
		return nil, err
	}
	return &repoIgnoreTester{repoRoot, g}, nil
}
