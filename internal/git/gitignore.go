package git

import (
	"context"
	"path/filepath"
	"strings"

	"github.com/monochromegane/go-gitignore"

	"github.com/windmilleng/tilt/internal/ospath"
)

// Known feature differences from git:
// 1. does not use git config core.excludesfile
// 2. only looks for .gitignore in repo root, instead of all directories between dirname(file) and repo root
// 3. does not use .git/info/exclude
// 4. does not take index into account

// ignores files specified in .gitignore
type gitIgnoreTester struct {
	repoRoot      string
	ignoreMatcher gitignore.IgnoreMatcher
}

func (i *gitIgnoreTester) Matches(f string, isDir bool) (bool, error) {
	if !ospath.IsChild(i.repoRoot, f) {
		return false, nil
	}

	return i.ignoreMatcher.Match(f, isDir), nil
}

func NewGitIgnoreTesterFromContents(ctx context.Context, repoRoot string, gitignoreContents string) (*gitIgnoreTester, error) {
	absRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return nil, err
	}

	i := gitignore.NewGitIgnoreFromReader(repoRoot, strings.NewReader(gitignoreContents))

	return &gitIgnoreTester{absRoot, i}, nil
}

// ignores files specified in .gitignore plus any files in $ROOT/.git/
type repoIgnoreTester struct {
	repoRoot        string
	gitIgnoreTester *gitIgnoreTester
}

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

func NewRepoIgnoreTester(ctx context.Context, repoRoot string, gitignoreContents string) (*repoIgnoreTester, error) {
	g, err := NewGitIgnoreTesterFromContents(ctx, repoRoot, gitignoreContents)
	if err != nil {
		return nil, err
	}
	return &repoIgnoreTester{repoRoot, g}, nil
}
