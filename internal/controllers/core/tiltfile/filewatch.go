package tiltfile

import (
	"fmt"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/tilt-dev/tilt/internal/ignore"
	"github.com/tilt-dev/tilt/internal/store"
	"github.com/tilt-dev/tilt/pkg/apis"
	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

type WatchInputs struct {
	TiltfileManifestName model.ManifestName
	TiltfilePath         string
	Manifests            []model.Manifest
	ConfigFiles          []string
	WatchSettings        model.WatchSettings
	Tiltignore           model.Dockerignore
	EngineMode           store.EngineMode
}

type WatchableTarget interface {
	ignore.IgnorableTarget
	Dependencies() []string
	ID() model.TargetID
}

var _ WatchableTarget = model.ImageTarget{}
var _ WatchableTarget = model.LocalTarget{}
var _ WatchableTarget = model.DockerComposeTarget{}

func specForTarget(t WatchableTarget, globalIgnores []model.Dockerignore) *v1alpha1.FileWatchSpec {
	watchedPaths := append([]string(nil), t.Dependencies()...)
	if len(watchedPaths) == 0 {
		return nil
	}

	spec := &v1alpha1.FileWatchSpec{
		WatchedPaths: watchedPaths,
		Ignores:      ignore.TargetToFileWatchIgnores(t),
	}

	// process global ignores last
	addGlobalIgnoresToSpec(spec, globalIgnores)

	return spec
}

func addGlobalIgnoresToSpec(spec *v1alpha1.FileWatchSpec, globalIgnores []model.Dockerignore) {
	for _, gi := range globalIgnores {
		spec.Ignores = append(spec.Ignores, v1alpha1.IgnoreDef{
			BasePath: gi.LocalPath,
			Patterns: append([]string(nil), gi.Patterns...),
		})
	}
}

// FileWatchesFromManifests creates FileWatch specs from Tilt manifests in the engine state.
func ToFileWatchObjects(watchInputs WatchInputs) typedObjectSet {
	result := typedObjectSet{}
	if !watchInputs.EngineMode.WatchesFiles() {
		return result
	}

	// TODO(milas): how can global ignores fit into the API model more cleanly?
	globalIgnores := globalIgnores(watchInputs)
	var fileWatches []*v1alpha1.FileWatch
	processedTargets := make(map[model.TargetID]bool)
	for _, m := range watchInputs.Manifests {
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
				fw := &v1alpha1.FileWatch{
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

	paths := []string{}
	if len(watchInputs.ConfigFiles) > 0 {
		paths = append(paths, watchInputs.ConfigFiles...)
	} else if watchInputs.TiltfilePath != "" {
		// A complete ConfigFiles set should include the Tiltfile. If it doesn't,
		// add it to the watch list now.
		paths = append(paths, watchInputs.TiltfilePath)
	}

	if len(paths) > 0 {
		id := fmt.Sprintf("%s:%s", model.TargetTypeConfigs, watchInputs.TiltfileManifestName)
		configFw := &v1alpha1.FileWatch{
			ObjectMeta: metav1.ObjectMeta{
				Name: apis.SanitizeName(id),
				Annotations: map[string]string{
					v1alpha1.AnnotationManifest: watchInputs.TiltfileManifestName.String(),
					v1alpha1.AnnotationTargetID: id,
				},
			},
			Spec: v1alpha1.FileWatchSpec{
				WatchedPaths: paths,
			},
		}

		addGlobalIgnoresToSpec(&configFw.Spec, globalIgnores)
		fileWatches = append(fileWatches, configFw)
	}

	for _, fw := range fileWatches {
		result[fw.Name] = fw
	}
	return result
}

// globalIgnores returns a list of global ignore patterns.
func globalIgnores(watchInputs WatchInputs) []model.Dockerignore {
	ignores := []model.Dockerignore{}
	if !watchInputs.Tiltignore.Empty() {
		ignores = append(ignores, watchInputs.Tiltignore)
	}
	ignores = append(ignores, watchInputs.WatchSettings.Ignores...)

	for _, manifest := range watchInputs.Manifests {
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
