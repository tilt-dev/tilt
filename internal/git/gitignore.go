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
	"github.com/windmilleng/tilt/internal/ospath"
)

// Known feature differences from git:
// 1. does not use git config core.excludesfile
// 2. only looks for .gitignore in repo root, instead of all directories between dirname(file) and repo root
// 3. does not use .git/info/exclude
// 4. does not take index into account

type IgnoreTester interface {
	IsIgnored(f string, isDir bool) (bool, error)
}

// an IgnoreTester that ignores nothing
type falseIgnoreTester struct{}

func (falseIgnoreTester) IsIgnored(f string, isDir bool) (bool, error) {
	return false, nil
}

var _ IgnoreTester = falseIgnoreTester{}

// ignores files specified in .gitignore
type gitIgnoreTester struct {
	repoRoot      string
	ignoreMatcher gitignore.IgnoreMatcher
}

var _ IgnoreTester = gitIgnoreTester{}

func (i gitIgnoreTester) IsIgnored(f string, isDir bool) (bool, error) {
	_, isChild := ospath.Child(i.repoRoot, f)
	if !isChild {
		return false, nil
	}

	return i.ignoreMatcher.Match(f, isDir), nil
}

func NewGitIgnoreTester(ctx context.Context, repoRoot string) (IgnoreTester, error) {
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
		//if it's any other error, log a warning and pretend the file doesn't exist (matching git's behavior)
		if ok && pathError.Err != syscall.ENOENT {
			logger.Get(ctx).Infof("warning: failed to open %V: %V", p, err)
		}
		return &falseIgnoreTester{}, nil
	}
	return &gitIgnoreTester{absRoot, i}, nil
}

// ignores files specified in .gitignore plus any files in $ROOT/.git/
type repoIgnoreTester struct {
	repoRoot        string
	gitIgnoreTester IgnoreTester
}

var _ IgnoreTester = repoIgnoreTester{}

func (r repoIgnoreTester) IsIgnored(f string, isDir bool) (bool, error) {
	absPath, err := filepath.Abs(f)
	if err != nil {
		return false, err
	}

	if strings.HasPrefix(absPath, filepath.Join(r.repoRoot, ".git/")) {
		return true, nil
	}

	return r.gitIgnoreTester.IsIgnored(f, isDir)
}

func NewRepoIgnoreTester(ctx context.Context, repoRoot string) (IgnoreTester, error) {
	g, err := NewGitIgnoreTester(ctx, repoRoot)
	if err != nil {
		return nil, err
	}
	return &repoIgnoreTester{repoRoot, g}, nil
}

type compositeIgnoreTester struct {
	testers []IgnoreTester
}

func (c compositeIgnoreTester) IsIgnored(f string, isDir bool) (bool, error) {
	for _, t := range c.testers {
		ret, err := t.IsIgnored(f, isDir)
		if err != nil {
			return false, err
		}
		if ret {
			return true, nil
		}
	}
	return false, nil
}

var _ IgnoreTester = compositeIgnoreTester{}

func NewMultiRepoIgnoreTester(ctx context.Context, repoRoots []string) (IgnoreTester, error) {
	var testers []IgnoreTester
	for _, repoRoot := range repoRoots {
		t, err := NewRepoIgnoreTester(ctx, repoRoot)
		if err != nil {
			return nil, err
		}

		testers = append(testers, t)
	}

	return compositeIgnoreTester{testers}, nil
}
