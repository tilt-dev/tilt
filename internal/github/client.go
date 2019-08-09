package github

import (
	"context"
	"strings"

	"github.com/google/go-github/github"
	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/pkg/model"
)

type Client interface {
	GetLatestRelease(ctx context.Context, org, repo string) (model.TiltBuild, error)
}

type ghClient struct {
	client *github.Client
}

func NewClient() Client {
	return ghClient{
		client: github.NewClient(nil),
	}
}

func (cli ghClient) GetLatestRelease(ctx context.Context, org, repo string) (model.TiltBuild, error) {
	release, _, err := cli.client.Repositories.GetLatestRelease(ctx, org, repo)
	if err != nil {
		return model.TiltBuild{}, errors.Wrapf(err, "error getting release for %s/%s", org, repo)
	}

	return model.TiltBuild{
		Version: strings.TrimPrefix(*release.Name, "v"),
		Date:    release.PublishedAt.Time.Format("2006-01-02"),
	}, nil
}
