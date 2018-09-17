package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/opentracing/opentracing-go"
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
// associates local filepaths with their mounts and destination paths).
func FilesToPathMappings(files []string, mounts []model.Mount) ([]pathMapping, error) {
	pms, err := filesToPathMappings(files, mounts)
	if err != nil {
		return pms, err
	}
	return pms, nil
}

func filesToPathMappings(files []string, mounts []model.Mount) ([]pathMapping, *PathMappingErr) {
	var pms []pathMapping
	for _, f := range files {
		pm, err := fileToPathMapping(f, mounts)
		if err != nil {
			return nil, err
		}
		pms = append(pms, pm)
	}

	return pms, nil
}

func fileToPathMapping(file string, mounts []model.Mount) (pathMapping, *PathMappingErr) {
	for _, m := range mounts {
		if !filepath.IsAbs(m.LocalPath) {
			return pathMapping{}, pathMappingErrf(
				"mount.LocalPath must be an absolute path (got: %s)",
				m.LocalPath)
		}
		// Open Q: can you mount inside of mounts?! o_0
		// TODO(maia): are symlinks etc. gonna kick our asses here? If so, will
		// need ospath.RealChild -- but then can't deal with deleted local files.
		relPath, isChild := ospath.Child(m.LocalPath, file)
		if isChild {
			return pathMapping{
				LocalPath:     file,
				ContainerPath: filepath.Join(m.ContainerPath, relPath),
			}, nil
		}
	}
	return pathMapping{}, pathMappingErrf("file %s matches no mounts", file)
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

type PathMappingErr struct {
	s string
}

func (e *PathMappingErr) Error() string { return e.s }

var _ error = &PathMappingErr{}

func pathMappingErrf(format string, a ...interface{}) *PathMappingErr {
	return &PathMappingErr{s: fmt.Sprintf(format, a...)}
}
