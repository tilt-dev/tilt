package lsp

import (
	"context"

	"go.lsp.dev/uri"

	"github.com/tilt-dev/starlark-lsp/pkg/document"

	"github.com/tilt-dev/tilt/internal/controllers/core/extension"
	"github.com/tilt-dev/tilt/internal/controllers/core/extensionrepo"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	"github.com/tilt-dev/tilt/internal/tiltfile/tiltextension"
	tiltfilev1alpha1 "github.com/tilt-dev/tilt/internal/tiltfile/v1alpha1"
)

type ExtensionFinder interface {
	ManagerOptions() []document.ManagerOpt
	Initialize(context context.Context, repo *extensionrepo.Reconciler, ext *extension.Reconciler)
}

func NewExtensionFinder() ExtensionFinder {
	return &extensionFinder{}
}

type extensionFinder struct {
	plugins     []starkit.Plugin
	interceptor starkit.LoadInterceptor
	ctx         context.Context
}

var _ ExtensionFinder = &extensionFinder{}

func (f *extensionFinder) Initialize(ctx context.Context, repo *extensionrepo.Reconciler, ext *extension.Reconciler) {
	f.ctx = ctx
	f.initializePlugins(tiltextension.NewPlugin(repo, ext))
}

func (f *extensionFinder) initializePlugins(extPlugin *tiltextension.Plugin) {
	f.plugins = []starkit.Plugin{
		extPlugin,
		tiltfilev1alpha1.NewPlugin(),
	}
	f.interceptor = extPlugin
}

func (f *extensionFinder) ManagerOptions() []document.ManagerOpt {
	return []document.ManagerOpt{
		document.WithReadDocumentFunc(f.readDocument),
		document.WithResolveURIFunc(f.resolveURI),
	}
}

func (f *extensionFinder) readDocument(u uri.URI) ([]byte, error) {
	path, err := f.extensionPath(string(u))

	if err != nil {
		return nil, err
	}

	if path != "" {
		u = uri.File(path)
	}

	return document.ReadDocument(u)
}

func (f *extensionFinder) extensionPath(module string) (string, error) {
	model, err := starkit.NewModel(f.plugins...)
	if err != nil {
		return "", err
	}
	thread := starkit.NewThread(f.ctx, model)
	return f.interceptor.LocalPath(thread, module)
}

func (f *extensionFinder) resolveURI(u uri.URI) (string, error) {
	path, err := f.extensionPath(string(u))
	if err != nil {
		return "", err
	}
	if path != "" {
		return path, nil
	}
	return document.ResolveURI(u)
}
