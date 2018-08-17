package build

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
)

// FileOp represents an operation we want to perform on the container -- either
// a. copying the file at LocalPath --> Containerpath, or
// b. if file at LocalPath doesn't exist, rm Containerpath
// Both LocalPath and ContainerPath are absolute paths
type FileOp struct {
	LocalPath     string
	ContainerPath string
}

// FilesToOps converts a list of absolute local filepaths into FileOps (i.e.
// associates local filepaths with their mounts and destination paths).
func FilesToOps(ctx context.Context, files []string, mounts []model.Mount) ([]FileOp, error) {
	var ops []FileOp
	for _, f := range files {
		foundMount := false
		for _, m := range mounts {
			// Open Q: can you mount inside of mounts?! o_0
			relPath, isChild, err := ospath.RealChild(m.Repo.LocalPath, f)
			if err != nil {
				return ops, fmt.Errorf("realChild: %v", err)
			}
			if isChild {
				foundMount = true
				ops = append(ops, FileOp{
					LocalPath:     f,
					ContainerPath: filepath.Join(m.ContainerPath, relPath),
				})
				break
			}
		}
		if !foundMount {
			logger.Get(ctx).Info(
				"Found no mount matching file %s, this probs shouldn't happen.", f)
		}

	}

	return ops, nil
}
