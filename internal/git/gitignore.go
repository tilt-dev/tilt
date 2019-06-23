package git

import (
	"context"
	"fmt"
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

	// match everything inside the .git/ directory
	gitPath := fmt.Sprintf("%s/", filepath.Join(r.repoRoot, ".git"))
	if strings.HasPrefix(absPath, gitPath) {
		return true, nil
	}

	// match the .git directory itself
	if strings.HasSuffix(absPath, ".git") {
		return true, nil
	}

	return false, nil
}

func NewRepoIgnoreTester(ctx context.Context, repoRoot string) (*repoIgnoreTester, error) {
	return &repoIgnoreTester{repoRoot}, nil
}
