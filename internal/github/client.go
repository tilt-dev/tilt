package github

import (
	"context"

	"github.com/google/go-github/github"
	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/model"
)

type Client interface {
	GetLatestRelease(ctx context.Context, org, repo string) (model.ReleaseVersion, error)
}

type ghClient struct {
	client *github.Client
}

func NewClient() Client {
	return ghClient{
		client: github.NewClient(nil),
	}
}

func (cli ghClient) GetLatestRelease(ctx context.Context, org, repo string) (model.ReleaseVersion, error) {
	release, _, err := cli.client.Repositories.GetLatestRelease(ctx, org, repo)
	if err != nil {
		return model.ReleaseVersion{}, errors.Wrapf(err, "error getting release for %s/%s", org, repo)
	}

	return model.ReleaseVersion{
		VersionNumber: *release.Name,
		PublishedAt:   release.PublishedAt.Time,
	}, nil
}
