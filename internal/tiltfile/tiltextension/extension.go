// Package extension implements Tilt extensions.
// This is not the internal Starkit abstraction, but the user-visible feature.
// In a Tiltfile, you can write `load("ext://foo", "bar")` to load the function bar
// from the extension foo.
package tiltextension

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
)

type Extension struct {
	fetcher Fetcher
	store   Store
	info    starkit.ExtensionsAnalyticsInfo
}

type Fetcher interface {
	Fetch(ctx context.Context, moduleName string) (ModuleContents, error)
	CleanUp() error
}

func NewExtension(fetcher Fetcher, store Store) *Extension {
	return &Extension{
		fetcher: fetcher,
		store:   store,
		info:    starkit.NewExtensionsAnalyticsInfo(),
	}
}

func ProvideExtension() (*Extension, error) {
	dlr, err := NewTempDirDownloader()
	if err != nil {
		return nil, err
	}
	fetcher := NewGithubFetcher(dlr)

	// Pass a nil store: we initialize it in OnStart once we know the dir of the Tiltfile
	return NewExtension(fetcher, nil), nil
}

func (e *Extension) OnStart(env *starkit.Environment) error {
	if e.store == nil {
		// If we haven't initialized the store yet, do it (now that we know
		// the path of the root Tiltfile)
		e.store = NewLocalStore(filepath.Dir(env.StartPath()))
	}
	env.AddLoadInterceptor(e)
	return nil
}

const extensionPrefix = "ext://"

func (e *Extension) LocalPath(t *starlark.Thread, arg string) (string, error) {
	ctx, err := starkit.ContextFromThread(t)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(arg, extensionPrefix) {
		return "", nil
	}

	moduleName := strings.TrimPrefix(arg, extensionPrefix)
	// If the module can't be found we fetch it below
	localPath, err := e.store.ModulePath(ctx, moduleName)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if localPath != "" {
		return localPath, nil
	}

	contents, err := e.fetcher.Fetch(ctx, moduleName)
	if err != nil {
		return "", err
	}

	defer func() {
		_ = e.fetcher.CleanUp()
	}()

	// Record that we loaded this extension
	e.info.ExtensionsLoaded[moduleName] = true

	return e.store.Write(ctx, contents)
}

func (e *Extension) AnalyticsInfo() []starkit.AnalyticsInfo {
	return []starkit.AnalyticsInfo{e.info}
}

var _ starkit.LoadInterceptor = (*Extension)(nil)
var _ starkit.Extension = (*Extension)(nil)
