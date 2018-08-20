package git

import (
	"path"
	"path/filepath"
	"strings"

	"github.com/monochromegane/go-gitignore"
)

type IgnoreTester interface {
	IsIgnored(f string, isDir bool) (bool, error)
}

type gitIgnoreTester struct {
	ignoreMatcher gitignore.IgnoreMatcher
}

var _ IgnoreTester = gitIgnoreTester{}

func (i gitIgnoreTester) IsIgnored(f string, isDir bool) (bool, error) {
	return i.ignoreMatcher.Match(f, isDir), nil
}

func NewGitIgnoreTester(repoRoot string) (IgnoreTester, error) {
	i, err := gitignore.NewGitIgnore(path.Join(repoRoot, ".gitignore"))
	if err != nil {
		return nil, err
	}
	return &gitIgnoreTester{i}, nil
}

type repoIgnoreTester struct {
	repoRoot string
	g        gitIgnoreTester
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

	return r.g.IsIgnored(f, isDir)
}

func NewRepoIgnoreTester(repoRoot string) (IgnoreTester, error) {
	g, err := NewGitIgnoreTester(repoRoot)
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

func NewMultiRepoIgnoreTester(repoRoots []string) (IgnoreTester, error) {
	var testers []IgnoreTester
	for _, repoRoot := range repoRoots {
		t, err := NewRepoIgnoreTester(repoRoot)
		if err != nil {
			return nil, err
		}

		testers = append(testers, t)
	}

	return compositeIgnoreTester{testers}, nil
}
