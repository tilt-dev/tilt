package fswatch

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/tilt-dev/tilt/internal/ignore"

	"github.com/tilt-dev/tilt/pkg/apis"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/tilt-dev/tilt/internal/store"
	filewatches "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	v1alpha1 "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

var ConfigsTargetID = model.TargetID{
	Type: model.TargetTypeConfigs,
	Name: "singleton",
}

type WatchableTarget interface {
	ignore.IgnorableTarget
	Dependencies() []string
	ID() model.TargetID
}

var _ WatchableTarget = model.ImageTarget{}
var _ WatchableTarget = model.LocalTarget{}
var _ WatchableTarget = model.DockerComposeTarget{}

// ManifestSubscriber watches the store for changes to manifests and creates/updates/deletes FileWatch objects.
type ManifestSubscriber struct {
	client ctrlclient.Client
}

func NewManifestSubscriber(client ctrlclient.Client) *ManifestSubscriber {
	return &ManifestSubscriber{
		client: client,
	}
}

func (w ManifestSubscriber) OnChange(ctx context.Context, st store.RStore, summary store.ChangeSummary) {
	if summary.IsLogOnly() || !summary.Legacy {
		return
	}

	state := st.RLockState()
	defer st.RUnlockState()

	if !state.EngineMode.WatchesFiles() {
		return
	}

	watchesToProcess := FileWatchesFromManifests(state)

	watchesToKeep := make(map[types.NamespacedName]bool)
	for _, fw := range watchesToProcess {
		name := types.NamespacedName{Namespace: fw.GetNamespace(), Name: fw.GetName()}
		watchesToKeep[name] = true

		existing := state.FileWatches[name]
		if existing != nil {
			if equality.Semantic.DeepEqual(existing.Spec, fw.Spec) {
				// spec has not changed
				continue
			}

			updated := existing.DeepCopy()
			fw.Spec.DeepCopyInto(&updated.Spec)
			err := w.client.Update(ctx, updated)
			if err == nil {
				st.Dispatch(NewFileWatchUpdateAction(updated))
			} else if !apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
				// conflict/not found errors are ignored; an update/delete must have happened against apiserver
				// that hasn't been processed by store yet; once processed, this handler will get run again at
				// which point things should be consistent (or repeat until such at time)
				// (if this were a real reconciler, it'd just explicitly request a requeue here)
				st.Dispatch(store.NewErrorAction(fmt.Errorf("apiserver update error: %v", err)))
				return
			}

		} else {
			err := w.client.Create(ctx, fw)
			if err == nil {
				st.Dispatch(NewFileWatchCreateAction(fw))
			} else if !apierrors.IsAlreadyExists(err) {
				st.Dispatch(store.NewErrorAction(fmt.Errorf("apiserver create error: %v", err)))
				return
			}
		}
	}

	// find and delete any that no longer exist from manifests
	for name, fw := range state.FileWatches {
		if _, ok := watchesToKeep[name]; !ok {
			toDelete := fw.DeepCopy()
			err := w.client.Delete(ctx, toDelete)
			if err == nil {
				st.Dispatch(NewFileWatchDeleteAction(name))
			} else if !apierrors.IsNotFound(err) {
				st.Dispatch(store.NewErrorAction(fmt.Errorf("apiserver delete error: %v", err)))
				return
			}
		}
	}
}

func specForTarget(t WatchableTarget, globalIgnores []model.Dockerignore) *filewatches.FileWatchSpec {
	watchedPaths := append([]string(nil), t.Dependencies()...)
	if len(watchedPaths) == 0 {
		return nil
	}

	spec := &filewatches.FileWatchSpec{
		WatchedPaths: watchedPaths,
	}

	for _, r := range t.LocalRepos() {
		spec.Ignores = append(spec.Ignores, filewatches.IgnoreDef{
			BasePath: filepath.Join(r.LocalPath, ".git"),
		})
	}

	for _, di := range t.Dockerignores() {
		if di.Empty() {
			continue
		}
		spec.Ignores = append(spec.Ignores, filewatches.IgnoreDef{
			BasePath: di.LocalPath,
			Patterns: append([]string(nil), di.Patterns...),
		})
	}
	for _, ild := range t.IgnoredLocalDirectories() {
		spec.Ignores = append(spec.Ignores, filewatches.IgnoreDef{
			BasePath: ild,
		})
	}

	// process global ignores last
	addGlobalIgnoresToSpec(spec, globalIgnores)

	return spec
}

func addGlobalIgnoresToSpec(spec *filewatches.FileWatchSpec, globalIgnores []model.Dockerignore) {
	for _, gi := range globalIgnores {
		spec.Ignores = append(spec.Ignores, filewatches.IgnoreDef{
			BasePath: gi.LocalPath,
			Patterns: append([]string(nil), gi.Patterns...),
		})
	}
}

// FileWatchesFromManifests creates FileWatch specs from Tilt manifests in the engine state.
func FileWatchesFromManifests(state store.EngineState) []*filewatches.FileWatch {
	// TODO(milas): how can global ignores fit into the API model more cleanly?
	globalIgnores := globalIgnores(state)
	var fileWatches []*filewatches.FileWatch
	processedTargets := make(map[model.TargetID]bool)
	for _, m := range state.Manifests() {
		for _, t := range m.TargetSpecs() {
			targetID := t.ID()
			// ignore targets that have already been processed or aren't watchable
			_, seen := processedTargets[targetID]
			t, ok := t.(WatchableTarget)
			if seen || !ok || targetID.Empty() {
				continue
			}
			processedTargets[targetID] = true
			spec := specForTarget(t, globalIgnores)
			if spec != nil {
				fw := &filewatches.FileWatch{
					ObjectMeta: metav1.ObjectMeta{
						Name: apis.SanitizeName(targetID.String()),
						Annotations: map[string]string{
							v1alpha1.AnnotationManifest: string(m.Name),
							v1alpha1.AnnotationTargetID: targetID.String(),
						},
					},
					Spec: *spec.DeepCopy(),
				}
				fileWatches = append(fileWatches, fw)
			}
		}
	}

	if len(state.ConfigFiles) > 0 {
		configFw := &filewatches.FileWatch{
			ObjectMeta: metav1.ObjectMeta{
				Name: apis.SanitizeName(ConfigsTargetID.String()),
				Annotations: map[string]string{
					v1alpha1.AnnotationTargetID: ConfigsTargetID.String(),
				},
			},
			Spec: filewatches.FileWatchSpec{
				WatchedPaths: append([]string(nil), state.ConfigFiles...),
			},
		}
		addGlobalIgnoresToSpec(&configFw.Spec, globalIgnores)
		fileWatches = append(fileWatches, configFw)
	}

	return fileWatches
}

// globalIgnores returns a list of global ignore patterns.
func globalIgnores(es store.EngineState) []model.Dockerignore {
	ignores := []model.Dockerignore{}
	if !es.Tiltignore.Empty() {
		ignores = append(ignores, es.Tiltignore)
	}
	ignores = append(ignores, es.WatchSettings.Ignores...)

	for _, manifest := range es.Manifests() {
		for _, iTarget := range manifest.ImageTargets {
			customBuild := iTarget.CustomBuildInfo()
			if customBuild.OutputsImageRefTo != "" {
				// this could be smarter and try to group by local path
				ignores = append(ignores, model.Dockerignore{
					LocalPath: filepath.Dir(customBuild.OutputsImageRefTo),
					Source:    "outputs_image_ref_to",
					Patterns:  []string{filepath.Base(customBuild.OutputsImageRefTo)},
				})
			}
		}
	}

	return ignores
}
