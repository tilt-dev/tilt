package git

import (
	"context"

	"github.com/windmilleng/tilt/internal/model"
)

func NewRepoIgnoreTester(ctx context.Context, repoRoot string) model.PathMatcher {
	return model.NewRelativeFileOrChildMatcher(repoRoot, ".git")
}
