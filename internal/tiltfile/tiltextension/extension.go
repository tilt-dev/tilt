// Package extension implements Tilt extensions.
// This is not the internal Starkit abstraction, but the user-visible feature.
// In a Tiltfile, you can write `load("ext://foo", "bar")` to load the function bar
// from the extension foo.
package tiltextension

import (
	"context"
	"os"
	"strings"

	"go.starlark.net/starlark"

	"github.com/tilt-dev/tilt/pkg/logger"

	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
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

type ExtState struct {
	ExtsLoaded map[string]bool
}

func (e Extension) NewState() interface{} {
	return ExtState{
		ExtsLoaded: make(map[string]bool),
	}
}

type Fetcher interface {
	Fetch(ctx context.Context, moduleName string) (ModuleContents, error)
	CleanUp() error
}

func (e *Extension) OnStart(env *starkit.Environment) error {
	env.AddLoadInterceptor(e)
	return nil
}

func (e *Extension) recordExtensionLoaded(ctx context.Context, t *starlark.Thread, moduleName string) {
	err := starkit.SetState(t, func(existing ExtState) (ExtState, error) {
		existing.ExtsLoaded[moduleName] = true
		return existing, nil
	})
	if err != nil {
		logger.Get(ctx).Debugf("error updating state on Tilt extensions loader: %v", err)
	}
}

const extensionPrefix = "ext://"

func (e *Extension) LocalPath(t *starlark.Thread, arg string) (localPath string, err error) {
	ctx, err := starkit.ContextFromThread(t)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(arg, extensionPrefix) {
		return "", nil
	}

	moduleName := strings.TrimPrefix(arg, extensionPrefix)
	defer func() {
		if err != nil {
			// NOTE(maia): Maybe in future we want to track if there was an error or not?
			// For now, only record on successful load.
			e.recordExtensionLoaded(ctx, t, moduleName)
		}
	}()

	// If the module can't be found we fetch it below
	localPath, err = e.store.ModulePath(ctx, moduleName)
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

	return e.store.Write(ctx, contents)
}

var _ starkit.LoadInterceptor = (*Extension)(nil)
var _ starkit.StatefulExtension = (*Extension)(nil)

func MustState(model starkit.Model) ExtState {
	state, err := GetState(model)
	if err != nil {
		panic(err)
	}
	return state
}

func GetState(m starkit.Model) (ExtState, error) {
	var state ExtState
	err := m.Load(&state)
	return state, err
}
