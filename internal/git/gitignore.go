package git

import (
	"github.com/tilt-dev/tilt/pkg/model"
)

// NewRepoIgnoreTester filters out changes in .git directories
func NewRepoIgnoreTester(repoRoot string) model.PathMatcher {
	return model.NewRelativeFileOrChildMatcher(repoRoot, ".git")
}
