package git

import (
	"github.com/windmilleng/tilt/pkg/model"
)

// NewRepoIgnoreTester filters out changes in .git directories
func NewRepoIgnoreTester(repoRoot string) model.PathMatcher {
	return model.NewRelativeFileOrChildMatcher(repoRoot, ".git")
}
