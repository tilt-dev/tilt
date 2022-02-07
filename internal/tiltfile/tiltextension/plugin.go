// Package extension implements Tilt extensions.
// This is not the internal Starkit abstraction, but the user-visible feature.
// In a Tiltfile, you can write `load("ext://foo", "bar")` to load the function bar
// from the extension foo.
package tiltextension

import (
	"context"
	"fmt"
	"strings"

	"go.starlark.net/starlark"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/controllers/apiset"
	"github.com/tilt-dev/tilt/internal/controllers/core/extension"
	"github.com/tilt-dev/tilt/internal/controllers/core/extensionrepo"
	"github.com/tilt-dev/tilt/internal/tiltfile/starkit"
	tiltfilev1alpha1 "github.com/tilt-dev/tilt/internal/tiltfile/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/logger"
)

const extensionPrefix = "ext://"
const defaultRepoName = "default"

type Plugin struct {
	repoReconciler ExtRepoReconciler
	extReconciler  ExtReconciler
}

func NewPlugin(repoReconciler *extensionrepo.Reconciler, extReconciler *extension.Reconciler) *Plugin {
	return &Plugin{
		repoReconciler: repoReconciler,
		extReconciler:  extReconciler,
	}
}

func NewFakePlugin(repoReconciler ExtRepoReconciler, extReconciler ExtReconciler) *Plugin {
	return &Plugin{
		repoReconciler: repoReconciler,
		extReconciler:  extReconciler,
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

	starkitModel, err := starkit.ModelFromThread(t)
	if err != nil {
		return "", err
	}

	objSet, err := tiltfilev1alpha1.GetState(starkitModel)
	if err != nil {
		return "", err
	}

	ext := e.ensureExtension(t, objSet, moduleName)
	repo := e.ensureRepo(t, objSet, ext.Spec.RepoName)
	repoStatus := e.repoReconciler.ForceApply(ctx, repo)
	if repoStatus.Error != "" {
		return "", fmt.Errorf("loading extension repo %s: %s", repo.Name, repoStatus.Error)
	}
	if repoStatus.Path == "" {
		return "", fmt.Errorf("extension repo not resolved: %s", repo.Name)
	}

	repoResolved := repo.DeepCopy()
	repoResolved.Status = repoStatus
	extStatus := e.extReconciler.ForceApply(ext, repoResolved)
	if extStatus.Error != "" {
		return "", fmt.Errorf("loading extension %s: %s", ext.Name, extStatus.Error)
	}
	if extStatus.Path == "" {
		return "", fmt.Errorf("extension not resolved: %s", ext.Name)
	}

	return extStatus.Path, nil
}

// Check to see if an extension has already been registered.
//
// If it has, returns the existing object (which should only have a spec).
//
// Otherwise, infers an extension object that points to the default repo.
func (e *Plugin) ensureExtension(t *starlark.Thread, objSet apiset.ObjectSet, moduleName string) *v1alpha1.Extension {
	extName := apis.SanitizeName(moduleName)
	defaultExt := &v1alpha1.Extension{
		ObjectMeta: metav1.ObjectMeta{
			Name: extName,
			Annotations: map[string]string{
				v1alpha1.AnnotationManagedBy: "tiltfile.loader",
			},
		},
		Spec: v1alpha1.ExtensionSpec{
			RepoName: defaultRepoName,
			RepoPath: moduleName,
		},
	}

	typedSet := objSet.GetOrCreateTypedSet(defaultExt)
	existing, exists := typedSet[extName]
	if exists {
		ext := existing.(*v1alpha1.Extension)
		metav1.SetMetaDataAnnotation(&ext.ObjectMeta, v1alpha1.AnnotationManagedBy, "tiltfile.loader")
		return ext
	}

	typedSet[extName] = defaultExt
	return defaultExt
}

// Check to see if an extension repo has already been registered.
//
// If it has, returns the existing object (which should only have a spec).
//
// Otherwise, register the default repo.
func (e *Plugin) ensureRepo(t *starlark.Thread, objSet apiset.ObjectSet, repoName string) *v1alpha1.ExtensionRepo {
	defaultRepo := &v1alpha1.ExtensionRepo{
		ObjectMeta: metav1.ObjectMeta{
			Name: repoName,
		},
		Spec: v1alpha1.ExtensionRepoSpec{
			URL: "https://github.com/tilt-dev/tilt-extensions",
			Ref: "HEAD",
		},
	}

	typedSet := objSet.GetOrCreateTypedSet(defaultRepo)
	existing, exists := typedSet[repoName]
	if exists {
		return existing.(*v1alpha1.ExtensionRepo)
	}

	typedSet[repoName] = defaultRepo
	return defaultRepo
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
