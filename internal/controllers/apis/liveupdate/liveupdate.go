package liveupdate

import (
	"path/filepath"

	"github.com/tilt-dev/tilt/pkg/apis/core/v1alpha1"
	"github.com/tilt-dev/tilt/pkg/model"
)

func IsEmptySpec(spec v1alpha1.LiveUpdateSpec) bool {
	return len(spec.Syncs) == 0 && len(spec.Execs) == 0
}

// FallBackOnFiles returns a PathSet of files which, if any have changed, indicate
// that we should fall back to an image build.
func FallBackOnFiles(spec v1alpha1.LiveUpdateSpec) model.PathSet {
	return model.NewPathSet(spec.StopPaths, spec.BasePath)
}

// Evaluates live-update syncs relative to the base path,
// and returns a sync with resolved paths.
func SyncSteps(spec v1alpha1.LiveUpdateSpec) []model.Sync {
	var syncs []model.Sync
	for _, sync := range spec.Syncs {
		localPath := sync.LocalPath
		if !filepath.IsAbs(localPath) {
			localPath = filepath.Join(spec.BasePath, localPath)
		}

		syncs = append(syncs, model.Sync{LocalPath: localPath, ContainerPath: sync.ContainerPath})
	}
	return syncs
}

// Evaluates live-update exec relative to the base path,
// and returns a run with resolved paths.
func RunSteps(spec v1alpha1.LiveUpdateSpec) []model.Run {
	var runs []model.Run
	for _, exec := range spec.Execs {
		runs = append(runs, model.Run{
			Cmd: model.Cmd{
				Argv: exec.Args,
			},
			Triggers: model.NewPathSet(exec.TriggerPaths, spec.BasePath),
		})
	}
	return runs
}

func ShouldRestart(spec v1alpha1.LiveUpdateSpec) bool {
	return spec.Restart == v1alpha1.LiveUpdateRestartStrategyAlways
}
