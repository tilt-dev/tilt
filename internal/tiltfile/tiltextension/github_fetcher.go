package tiltextension

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/google/go-github/v29/github"

	pkgtiltextension "github.com/windmilleng/tilt/pkg/tiltextension"
)

type GithubFetcher struct {
}

// TODO(dmiller): DI github
// TODO(dmiller): DI HTTP client
func NewGithubFetcher() *GithubFetcher {
	return &GithubFetcher{}
}

const githubTemplate = "https://raw.githubusercontent.com/windmilleng/tilt-extensions/%s/%s/Tiltfile"

func (f *GithubFetcher) Fetch(ctx context.Context, moduleName string) (ModuleContents, error) {
	client := github.NewClient(nil)
	masterBranch, _, err := client.Repositories.GetBranch(ctx, "windmilleng", "tilt-extensions", "master")
	if err != nil {
		return ModuleContents{}, err
	}
	headOfMaster := masterBranch.GetCommit()
	masterSHA := headOfMaster.GetSHA()

	err = pkgtiltextension.ValidateName(moduleName)
	if err != nil {
		return ModuleContents{}, err
	}
	c := &http.Client{
		Timeout: 20 * time.Second,
	}

	urlText := fmt.Sprintf(githubTemplate, masterSHA, moduleName)
	resp, err := c.Get(urlText)
	if err != nil {
		return ModuleContents{}, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ModuleContents{}, fmt.Errorf("error fetching Tiltfile %q: %v", urlText, resp.Status)
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return ModuleContents{}, err
	}

	return ModuleContents{
		Name:              moduleName,
		TiltfileContents:  string(body),
		GitCommitHash:     masterSHA,
		ExtensionRegistry: "https://github.com/windmilleng/tilt-extensions",
		TimeFetched:       time.Now(),
	}, nil
}

var _ Fetcher = (*GithubFetcher)(nil)
