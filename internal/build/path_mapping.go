package build

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
)

// pathMapping represents a mapping from a local path to the path on a container
// where it should be mounted. Both LocalPath and ContainerPath are absolute paths.
// May be files or directories.
type pathMapping struct {
	LocalPath     string
	ContainerPath string
}

// FilesToPathMappings converts a list of absolute local filepaths into pathMappings (i.e.
// associates local filepaths with their mounts and destination paths).
func FilesToPathMappings(files []string, mounts []model.Mount) ([]pathMapping, error) {
	var pms []pathMapping
	for _, f := range files {
		foundMount := false
		for _, m := range mounts {
			// Open Q: can you mount inside of mounts?! o_0
			// TODO(maia): are symlinks etc. gonna kick our asses here? If so, will
			// need ospath.RealChild -- but then can't deal with deleted local files.
			relPath, isChild := ospath.Child(m.Repo.LocalPath, f)
			if isChild {
				foundMount = true
				pms = append(pms, pathMapping{
					LocalPath:     f,
					ContainerPath: filepath.Join(m.ContainerPath, relPath),
				})
				break
			}
		}
		if !foundMount {
			return nil, fmt.Errorf("file %s matches no mounts", f)
		}

	}

	return pms, nil
}

func MountsToPath(mounts []model.Mount) []pathMapping {
	pms := make([]pathMapping, len(mounts))
	for i, m := range mounts {
		pms[i] = pathMapping{
			LocalPath:     m.Repo.LocalPath,
			ContainerPath: m.ContainerPath,
		}
	}
	return pms
}

// Return all the path mappings for local paths that do not exist.
func missingLocalPaths(mappings []pathMapping) ([]pathMapping, error) {
	result := make([]pathMapping, 0)
	for _, mapping := range mappings {
		_, err := os.Stat(mapping.LocalPath)
		if err == nil {
			continue
		}

		if os.IsNotExist(err) {
			result = append(result, mapping)
		} else {
			return nil, fmt.Errorf("missingLocalPaths: %v", err)
		}
	}
	return result, nil
}
