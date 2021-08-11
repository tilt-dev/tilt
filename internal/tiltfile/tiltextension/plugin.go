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

type Plugin struct {
	fetcher Fetcher
	store   Store
}

func NewPlugin(fetcher Fetcher, store Store) *Plugin {
	return &Plugin{
		fetcher: fetcher,
		store:   store,
	}
}

type State struct {
	ExtsLoaded map[string]bool
}

func (e Plugin) NewState() interface{} {
	return State{
		ExtsLoaded: make(map[string]bool),
	}
}

type Fetcher interface {
	Fetch(ctx context.Context, moduleName string) (ModuleContents, error)
	CleanUp() error
}

func (e *Plugin) OnStart(env *starkit.Environment) error {
	env.AddLoadInterceptor(e)
	return nil
}

func (e *Plugin) recordExtensionLoaded(ctx context.Context, t *starlark.Thread, moduleName string) {
	err := starkit.SetState(t, func(existing State) (State, error) {
		existing.ExtsLoaded[moduleName] = true
		return existing, nil
	})
	if err != nil {
		logger.Get(ctx).Debugf("error updating state on Tilt extensions loader: %v", err)
	}
}

const extensionPrefix = "ext://"

func (e *Plugin) LocalPath(t *starlark.Thread, arg string) (localPath string, err error) {
	ctx, err := starkit.ContextFromThread(t)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(arg, extensionPrefix) {
		return "", nil
	}

	moduleName := strings.TrimPrefix(arg, extensionPrefix)
	defer func() {
		if err == nil {
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

var _ starkit.LoadInterceptor = (*Plugin)(nil)
var _ starkit.StatefulPlugin = (*Plugin)(nil)

func MustState(model starkit.Model) State {
	state, err := GetState(model)
	if err != nil {
		panic(err)
	}
	return state
}

func GetState(m starkit.Model) (State, error) {
	var state State
	err := m.Load(&state)
	return state, err
}
