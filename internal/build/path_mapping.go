package build

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/ospath"
	"github.com/windmilleng/tilt/pkg/model"
)

// PathMapping represents a mapping from the local path to the tarball path
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
// In DockerBuilds, this is no longer the case.
//
// TODO(nick): Do a pass on renaming all the path types
type PathMapping struct {
	LocalPath     string
	ContainerPath string
}

func (m PathMapping) PrettyStr() string {
	return fmt.Sprintf("'%s' --> '%s'", m.LocalPath, m.ContainerPath)
}

func (m PathMapping) Filter(matcher model.PathMatcher) ([]PathMapping, error) {
	result := make([]PathMapping, 0)
	err := filepath.Walk(m.LocalPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		match, err := matcher.Matches(path)
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

		result = append(result, PathMapping{
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

func FilterMappings(mappings []PathMapping, matcher model.PathMatcher) ([]PathMapping, error) {
	result := make([]PathMapping, 0)
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
// associates local filepaths with their syncs and destination paths), returning those
// that it cannot associate with a sync.
func FilesToPathMappings(files []string, syncs []model.Sync) ([]PathMapping, []string, error) {
	pms := make([]PathMapping, 0, len(files))
	pathsMatchingNoSync := []string{}
	for _, f := range files {
		pm, couldMap, err := fileToPathMapping(f, syncs)
		if err != nil {
			return nil, nil, err
		}

		if couldMap {
			pms = append(pms, pm)
		} else {
			pathsMatchingNoSync = append(pathsMatchingNoSync, f)
		}
	}

	return pms, pathsMatchingNoSync, nil
}

func fileToPathMapping(file string, sync []model.Sync) (pm PathMapping, couldMap bool, err error) {
	for _, s := range sync {
		// Open Q: can you sync files inside of syncs?! o_0
		// TODO(maia): are symlinks etc. gonna kick our asses here? If so, will
		// need ospath.RealChild -- but then can't deal with deleted local files.
		relPath, isChild := ospath.Child(s.LocalPath, file)
		if isChild {
			localPathIsFile, err := isFile(s.LocalPath)
			if err != nil {
				return PathMapping{}, false, fmt.Errorf("error stat'ing: %v", err)
			}
			var containerPath string
			if endsWithSlash(s.ContainerPath) && localPathIsFile {
				fileName := path.Base(s.LocalPath)
				containerPath = filepath.Join(s.ContainerPath, fileName)
			} else {
				containerPath = filepath.Join(s.ContainerPath, relPath)
			}
			return PathMapping{
				LocalPath:     file,
				ContainerPath: containerPath,
			}, true, nil
		}
	}
	// (Potentially) expected case: the file doesn't match any sync src's. It's up
	// to the caller to decide whether this is expected or not.
	// E.g. for LiveUpdate, this is an expected case; for FastBuild, it means
	// something is wrong (as we only WATCH files/dirs specified by the sync's).
	return PathMapping{}, false, nil
}

func endsWithSlash(path string) bool {
	return strings.HasSuffix(path, string(filepath.Separator))
}

func isFile(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	mode := fi.Mode()
	return !mode.IsDir(), nil
}

func SyncsToPathMappings(syncs []model.Sync) []PathMapping {
	pms := make([]PathMapping, len(syncs))
	for i, s := range syncs {
		pms[i] = PathMapping{
			LocalPath:     s.LocalPath,
			ContainerPath: s.ContainerPath,
		}
	}
	return pms
}

// Return all the path mappings for local paths that do not exist.
func MissingLocalPaths(ctx context.Context, mappings []PathMapping) (missing, rest []PathMapping, err error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "MissingLocalPaths")
	_ = ctx
	defer span.Finish()
	for _, mapping := range mappings {
		_, err := os.Stat(mapping.LocalPath)
		if err == nil {
			rest = append(rest, mapping)
			continue
		}

		if os.IsNotExist(err) {
			missing = append(missing, mapping)
		} else {
			return nil, nil, errors.Wrap(err, "MissingLocalPaths")
		}
	}
	return missing, rest, nil
}

func PathMappingsToContainerPaths(mappings []PathMapping) []string {
	res := make([]string, len(mappings))
	for i, m := range mappings {
		res[i] = m.ContainerPath
	}
	return res
}

func PathMappingsToLocalPaths(mappings []PathMapping) []string {
	res := make([]string, len(mappings))
	for i, m := range mappings {
		res[i] = m.LocalPath
	}
	return res
}
