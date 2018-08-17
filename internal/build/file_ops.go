package build

import (
	"context"
	"path/filepath"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
)

// PathMapping represents a mapping from a local path to the path on a container
// where it should be mounted. Both LocalPath and ContainerPath are absolute paths.
type PathMapping struct {
	LocalPath     string
	ContainerPath string
}

// FilesToPathMappings converts a list of absolute local filepaths into FileOps (i.e.
// associates local filepaths with their mounts and destination paths).
func FilesToPathMappings(ctx context.Context, files []string, mounts []model.Mount) []PathMapping {
	var pms []PathMapping
	for _, f := range files {
		foundMount := false
		for _, m := range mounts {
			// Open Q: can you mount inside of mounts?! o_0
			// TODO(maia): are symlinks etc. gonna kick our asses here? If so, will
			// need ospath.RealChild -- but then can't deal with deleted local files.
			relPath, isChild := ospath.Child(m.Repo.LocalPath, f)
			if isChild {
				foundMount = true
				pms = append(pms, PathMapping{
					LocalPath:     f,
					ContainerPath: filepath.Join(m.ContainerPath, relPath),
				})
				break
			}
		}
		if !foundMount {
			// TODO(maia) should maybe be returned as an error, depending on the contract
			// with the file watcher.
			logger.Get(ctx).Info(
				"Found no mount matching file %s, this probs shouldn't happen.", f)
		}

	}

	return pms
}
