package fswatch

import (
	"context"
	"path/filepath"

	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/tilt-dev/tilt/internal/store"
	filewatches "github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

// ManifestSubscriber watches the store for changes to manifests and creates/updates/deletes FileWatch objects.
type ManifestSubscriber struct{}

func NewManifestSubscriber() *ManifestSubscriber {
	return &ManifestSubscriber{}
}

func (w ManifestSubscriber) OnChange(_ context.Context, st store.RStore, _ store.ChangeSummary) {
	state := st.RLockState()
	defer st.RUnlockState()

	if !state.EngineMode.WatchesFiles() {
		return
	}

	// TODO(milas): how can global ignores fit into the API model more cleanly?
	newGlobalIgnores := globalIgnores(state)
	manifests := state.Manifests()
	specsToProcess := SpecsForManifests(manifests, newGlobalIgnores)

	if len(state.ConfigFiles) > 0 {
		specsToProcess[ConfigsTargetID] = filewatches.FileWatchSpec{
			WatchedPaths: state.ConfigFiles,
		}
	}

	watchesToKeep := make(map[types.NamespacedName]bool)
	for targetID, spec := range specsToProcess {
		name := types.NamespacedName{Name: targetID.String()}
		watchesToKeep[name] = true

		existing := state.FileWatches[name]
		if existing != nil {
			if equality.Semantic.DeepEqual(existing.Spec, spec) {
				// spec has not changed
				continue
			}

			updated := existing.DeepCopy()
			spec.DeepCopyInto(&updated.Spec)
			st.Dispatch(NewFileWatchUpdateAction(updated))
		} else {
			fw := &filewatches.FileWatch{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: name.Namespace,
					Name:      name.Name,
					Labels: map[string]string{
						filewatches.LabelTargetID: targetID.String(),
					},
				},
				Spec: *spec.DeepCopy(),
			}
			st.Dispatch(NewFileWatchCreateAction(fw))
		}
	}

	// find and delete any that no longer exist from manifests
	for name := range state.FileWatches {
		if _, ok := watchesToKeep[name]; !ok {
			st.Dispatch(NewFileWatchDeleteAction(name))
		}
	}
}

func specForTarget(t WatchableTarget, globalIgnores []model.Dockerignore) filewatches.FileWatchSpec {
	spec := filewatches.FileWatchSpec{
		WatchedPaths: append([]string{}, t.Dependencies()...),
	}
	for _, di := range t.Dockerignores() {
		if di.Empty() {
			continue
		}
		spec.Ignores = append(spec.Ignores, filewatches.IgnoreDef{
			BasePath: di.LocalPath,
			Patterns: di.Patterns,
		})
	}
	for _, ild := range t.IgnoredLocalDirectories() {
		spec.Ignores = append(spec.Ignores, filewatches.IgnoreDef{
			BasePath: ild,
		})
	}

	// process global ignores last
	for _, gi := range globalIgnores {
		spec.Ignores = append(spec.Ignores, filewatches.IgnoreDef{
			BasePath: gi.LocalPath,
			Patterns: gi.Patterns,
		})
	}

	return spec
}

// SpecsForManifests creates FileWatch specs from Tilt manifests.
func SpecsForManifests(manifests []model.Manifest, globalIgnores []model.Dockerignore) map[model.TargetID]filewatches.FileWatchSpec {
	fileWatches := make(map[model.TargetID]filewatches.FileWatchSpec)
	for _, m := range manifests {
		for _, t := range m.TargetSpecs() {
			// ignore targets that have already been processed or aren't watchable
			_, seen := fileWatches[t.ID()]
			t, ok := t.(WatchableTarget)
			if seen || !ok {
				continue
			}
			fileWatches[t.ID()] = specForTarget(t, globalIgnores)
		}
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
