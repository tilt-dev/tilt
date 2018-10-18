package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/opentracing/opentracing-go"
	"github.com/windmilleng/tilt/internal/logger"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/ospath"
)

// pathMapping represents a mapping from the local path to the tarball path
//
// To send a local file into a container, we copy it into a tarball, send the
// tarball to docker, and then run a sequence of steps to unpack the tarball in
// the container file system.
//
// That means every file has 3 paths:
// 1) LocalPath
// 2) TarballPath
// 3) ContainerPath
//
// In incremental builds, TarballPath and ContainerPath are always the
// same, so it was correct to use TarballPath and ContainerPath interchangeably.
//
// In static builds, this is no longer the case.
//
// TODO(nick): Do a pass on renaming all the path types
type pathMapping struct {
	LocalPath     string
	ContainerPath string
}

func (m pathMapping) Empty() bool { return m.LocalPath == "" && m.ContainerPath == "" }

func (m pathMapping) Filter(matcher model.PathMatcher) ([]pathMapping, error) {
	result := make([]pathMapping, 0)
	err := filepath.Walk(m.LocalPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		match, err := matcher.Matches(path, info.IsDir())
		if err != nil {
			return err
		}

		if !match {
			return nil
		}

		rp, err := filepath.Rel(m.LocalPath, path)
		if err != nil {
			return err
		}

		result = append(result, pathMapping{
			LocalPath:     path,
			ContainerPath: filepath.Join(m.ContainerPath, rp),
		})
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}

func FilterMappings(mappings []pathMapping, matcher model.PathMatcher) ([]pathMapping, error) {
	result := make([]pathMapping, 0)
	for _, mapping := range mappings {
		filtered, err := mapping.Filter(matcher)
		if err != nil {
			return nil, err
		}

		result = append(result, filtered...)
	}
	return result, nil
}

// FilesToPathMappings converts a list of absolute local filepaths into pathMappings (i.e.
// associates local filepaths with their mounts and destination paths). If a file does
// not belong to a mapping, ignore it.
func FilesToPathMappings(ctx context.Context, files []string, mounts []model.Mount) []pathMapping {
	var pms []pathMapping
	for _, f := range files {
		pm := fileToPathMapping(f, mounts)
		if pm.Empty() {
			logger.Get(ctx).Debugf("file '%s' matches no mounts, skipping", f)
			continue
		}
		pms = append(pms, pm)
	}

	return pms
}

func fileToPathMapping(file string, mounts []model.Mount) pathMapping {
	for _, m := range mounts {
		// Open Q: can you mount inside of mounts?! o_0
		// TODO(maia): are symlinks etc. gonna kick our asses here? If so, will
		// need ospath.RealChild -- but then can't deal with deleted local files.
		relPath, isChild := ospath.Child(m.LocalPath, file)
		if isChild {
			return pathMapping{
				LocalPath:     file,
				ContainerPath: filepath.Join(m.ContainerPath, relPath),
			}
		}
	}

	// File didn't match any mounts
	return pathMapping{}
}

func MountsToPathMappings(mounts []model.Mount) []pathMapping {
	pms := make([]pathMapping, len(mounts))
	for i, m := range mounts {
		pms[i] = pathMapping{
			LocalPath:     m.LocalPath,
			ContainerPath: m.ContainerPath,
		}
	}
	return pms
}

// Return all the path mappings for local paths that do not exist.
func MissingLocalPaths(ctx context.Context, mappings []pathMapping) ([]pathMapping, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "MissingLocalPaths")
	defer span.Finish()
	result := make([]pathMapping, 0)
	for _, mapping := range mappings {
		_, err := os.Stat(mapping.LocalPath)
		if err == nil {
			continue
		}

		if os.IsNotExist(err) {
			result = append(result, mapping)
		} else {
			return nil, fmt.Errorf("MissingLocalPaths: %v", err)
		}
	}
	return result, nil
}

func PathMappingsToContainerPaths(mappings []pathMapping) []string {
	res := make([]string, len(mappings))
	for i, m := range mappings {
		res[i] = m.ContainerPath
	}
	return res
}
