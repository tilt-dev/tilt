package git

import (
	"context"

	"github.com/windmilleng/tilt/pkg/model"
)

func NewRepoIgnoreTester(ctx context.Context, repoRoot string) model.PathMatcher {
	return model.NewRelativeFileOrChildMatcher(repoRoot, ".git")
}
