package tiltextension

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/tilt-dev/go-get"

	"github.com/tilt-dev/tilt/internal/watch"
	"github.com/tilt-dev/tilt/pkg/logger"
)

type Downloader interface {
	RootDir() string
	Download(ctx context.Context, pkg string) (string, error)
}

type TempDirDownloader struct {
	rootDir string
}

func NewTempDirDownloader(dir *watch.TempDir) (*TempDirDownloader, error) {
	extdir, err := dir.NewDir("tilt-extensions")
	if err != nil {
		return nil, err
	}
	return &TempDirDownloader{rootDir: extdir.Path()}, nil
}

func (d *TempDirDownloader) RootDir() string {
	return d.rootDir
}

func (d *TempDirDownloader) Download(ctx context.Context, pkg string) (string, error) {
	dlr := get.NewDownloader(d.rootDir)
	dlr.Stderr = logger.Get(ctx).Writer(logger.InfoLvl)

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
	dir, err := f.dlr.Download(ctx, path.Join("github.com/tilt-dev/tilt-extensions", moduleName))
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
