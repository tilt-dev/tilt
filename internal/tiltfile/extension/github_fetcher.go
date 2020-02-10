package extension

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

type GithubFetcher struct {
}

func NewGithubFetcher() *GithubFetcher {
	return &GithubFetcher{}
}

const githubTemplate = "https://raw.githubusercontent.com/windmilleng/tilt-extensions/master/%s/Tiltfile"

func (f *GithubFetcher) Fetch(ctx context.Context, moduleName string) (ModuleContents, error) {
	c := &http.Client{
		Timeout: 20 * time.Second,
	}

	urlText := fmt.Sprintf(githubTemplate, url.PathEscape(moduleName))
	resp, err := c.Get(urlText)
	if err != nil {
		return ModuleContents{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ModuleContents{}, fmt.Errorf("error fetching Tiltfile %q: %v", urlText, err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ModuleContents{}, err
	}

	return ModuleContents{
		Name:             moduleName,
		TiltfileContents: string(body),
	}, nil
}

var _ Fetcher = (*GithubFetcher)(nil)
