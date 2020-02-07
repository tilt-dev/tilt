// Package extension implements Tilt extensions.
// This is not the internal Starkit abstraction, but the user-visible feature.
// In a Tiltfile, you can write `load("ext://foo", "bar")` to load the function bar
// from the extension foo.
package extension

import (
	"context"
	"os"
	"strings"

	"go.starlark.net/starlark"

	"github.com/windmilleng/tilt/internal/tiltfile/starkit"
)

type Extension struct {
	ctx     context.Context
	fetcher Fetcher
	store   Store
}

func NewExtension(ctx context.Context, fetcher Fetcher, store Store) *Extension {
	return &Extension{
		ctx:     ctx,
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
	if !strings.HasPrefix(arg, extensionPrefix) {
		return "", nil
	}

	moduleName := strings.TrimPrefix(arg, extensionPrefix)
	// If the module can't be found we fetch it below
	localPath, err := e.store.ModulePath(e.ctx, moduleName)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if localPath != "" {
		return localPath, nil
	}

	contents, err := e.fetcher.Fetch(e.ctx, moduleName)
	if err != nil {
		return "", err
	}

	return e.store.Write(e.ctx, contents)
}

var _ starkit.LoadInterceptor = (*Extension)(nil)
var _ starkit.Extension = (*Extension)(nil)
