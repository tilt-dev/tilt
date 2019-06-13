package git

import (
	"context"
	"path/filepath"
	"strings"
)

// ignores files specified in $ROOT/.git/
type repoIgnoreTester struct {
	repoRoot string
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

	return false, nil
}

func NewRepoIgnoreTester(ctx context.Context, repoRoot string) (*repoIgnoreTester, error) {
	return &repoIgnoreTester{repoRoot}, nil
}
