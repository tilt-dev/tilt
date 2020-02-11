// Package extension implements Tilt extensions.
// This is not the internal Starkit abstraction, but the user-visible feature.
// In a Tiltfile, you can write `load("ext://foo", "bar")` to load the function bar
// from the extension foo.
package tiltextension

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

type Extension struct {
	fetcher Fetcher
	store   Store
}

func NewExtension(fetcher Fetcher, store Store) *Extension {
	return &Extension{
		fetcher: fetcher,
		store:   store,
	}
}

type Fetcher interface {
	Fetch(ctx context.Context, moduleName string) (ModuleContents, error)
}

func (e *Extension) OnStart(env *starkit.Environment) error {
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

	loadIsHappeningInTopLevel := t.CallStackDepth() == 1

	if !loadIsHappeningInTopLevel {
		return "", fmt.Errorf("extensions cannot be loaded from `load`ed Tiltfiles")
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

	return e.store.Write(ctx, contents)
}

var _ starkit.LoadInterceptor = (*Extension)(nil)
var _ starkit.Extension = (*Extension)(nil)
