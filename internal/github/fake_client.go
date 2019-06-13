package github

import (
	"context"

	"github.com/windmilleng/tilt/internal/model"
)

type FakeClient struct {
	LatestReleaseRet model.TiltBuild
	LatestReleaseErr error
}

func (fc *FakeClient) GetLatestRelease(ctx context.Context, org, repo string) (model.TiltBuild, error) {
	return fc.LatestReleaseRet, fc.LatestReleaseErr
}

var _ Client = &FakeClient{}
