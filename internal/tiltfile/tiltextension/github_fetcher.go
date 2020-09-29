package tiltextension

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"time"

	"github.com/tilt-dev/go-get"
)

type Downloader interface {
	RootDir() string
	Download(pkg string) (string, error)
}

type TempDirDownloader struct {
	rootDir string
}

func NewTempDirDownloader() (*TempDirDownloader, error) {
	dir, err := ioutil.TempDir("", "tilt-extensions")
	if err != nil {
		return nil, err
	}
	return &TempDirDownloader{rootDir: dir}, nil
}

func (d *TempDirDownloader) RootDir() string {
	return d.rootDir
}

func (d *TempDirDownloader) Download(pkg string) (string, error) {
	dlr := get.NewDownloader(d.rootDir)
	return dlr.Download(pkg)
}

type GithubFetcher struct {
	dlr Downloader
}

func NewGithubFetcher(dlr Downloader) *GithubFetcher {
	return &GithubFetcher{dlr: dlr}
}

func (f *GithubFetcher) CleanUp() error {
	return os.RemoveAll(f.dlr.RootDir())
}

func (f *GithubFetcher) Fetch(ctx context.Context, moduleName string) (ModuleContents, error) {
	dir, err := f.dlr.Download(path.Join("github.com/tilt-dev/tilt-extensions", moduleName))
	if err != nil {
		return ModuleContents{}, fmt.Errorf("Fetching tilt-extensions: %v", err)
	}

	return ModuleContents{
		Name:              moduleName,
		Dir:               dir,
		ExtensionRegistry: "https://github.com/tilt-dev/tilt-extensions",
		TimeFetched:       time.Now(),
	}, nil
}

var _ Fetcher = (*GithubFetcher)(nil)
