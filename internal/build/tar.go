package build

import (
	"archive/tar"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"

	"github.com/windmilleng/tilt/internal/dockerfile"
	"github.com/windmilleng/tilt/pkg/model"
)

type ArchiveBuilder struct {
	tw     *tar.Writer
	filter model.PathMatcher
	paths  []string // local paths archived
}

func NewArchiveBuilder(writer io.Writer, filter model.PathMatcher) *ArchiveBuilder {
	tw := tar.NewWriter(writer)
	if filter == nil {
		filter = model.EmptyMatcher
	}

	return &ArchiveBuilder{tw: tw, filter: filter}
}

func (a *ArchiveBuilder) Close() error {
	return a.tw.Close()
}

// NOTE(dmiller) sometimes users will have very large UID/GIDs that will cause
// archive/tar to switch to PAX format, which will trip this Docker bug:
// https://github.com/docker/cli/issues/1459
// To prevent this, simply clear these out before adding to tar.
func clearUIDAndGID(h *tar.Header) {
	h.Uid = 0
	h.Gid = 0
}

func (a *ArchiveBuilder) archiveDf(ctx context.Context, df dockerfile.Dockerfile) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-archiveDf")
	_ = ctx
	defer span.Finish()
	tarHeader := &tar.Header{
		Name:       "Dockerfile",
		Typeflag:   tar.TypeReg,
		Size:       int64(len(df)),
		ModTime:    time.Now(),
		AccessTime: time.Now(),
		ChangeTime: time.Now(),
	}
	clearUIDAndGID(tarHeader)
	err := a.tw.WriteHeader(tarHeader)
	if err != nil {
		return err
	}
	_, err = a.tw.Write([]byte(df))
	if err != nil {
		return err
	}

	return nil
}

// ArchivePathsIfExist creates a tar archive of all local files in `paths`. It quietly skips any paths that don't exist.
func (a *ArchiveBuilder) ArchivePathsIfExist(ctx context.Context, paths []PathMapping) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-ArchivePathsIfExist")
	defer span.Finish()

	// In order to handle overlapping syncs, we
	// 1) collect all the entries,
	// 2) de-dupe them, with last-one-wins semantics
	// 3) write all the entries
	//
	// It's not obvious that this is the correct behavior. A better approach
	// (that's more in-line with how syncs work) might ignore files in earlier
	// path mappings when we know they're going to be "synced" over.
	// There's a bunch of subtle product decisions about how overlapping path
	// mappings work that we're not sure about.
	entries := []archiveEntry{}
	for _, p := range paths {
		newEntries, err := a.entriesForPath(ctx, p.LocalPath, p.ContainerPath)
		if err != nil {
			return errors.Wrapf(err, "tarPath '%s'", p.LocalPath)
		}

		entries = append(entries, newEntries...)
	}

	entries = dedupeEntries(entries)
	for _, entry := range entries {
		err := a.writeEntry(entry)
		if err != nil {
			return errors.Wrapf(err, "tarPath '%s'", entry.path)
		}
		a.paths = append(a.paths, entry.path)
	}
	return nil
}

// Local paths that were archived
func (a *ArchiveBuilder) Paths() []string {
	return a.paths
}

type archiveEntry struct {
	path   string
	info   os.FileInfo
	header *tar.Header
}

// tarPath writes the given source path into tarWriter at the given dest (recursively for directories).
// e.g. tarring my_dir --> dest d: d/file_a, d/file_b
// If source path does not exist, quietly skips it and returns no err
func (a *ArchiveBuilder) entriesForPath(ctx context.Context, source, dest string) ([]archiveEntry, error) {
	// (PL) convert \ to / in case of windows path
	source = filepath.ToSlash(source)

	sourceInfo, err := os.Stat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, errors.Wrapf(err, "%s: stat", source)
	}

	sourceIsDir := sourceInfo.IsDir()
	if sourceIsDir {
		// Make sure we can trim this off filenames to get valid relative filepaths
		if !strings.HasSuffix(source, "/") {
			source += "/"
		}
	}

	// (PL) convert \ to / in case of windows path
	dest = filepath.ToSlash(dest)

	// (PL) remove volume name + / so if we got a windows path C:\foo it becomes foo
	dest = strings.TrimPrefix(dest, filepath.VolumeName(dest)+"/")

	result := make([]archiveEntry, 0)
	err = filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return errors.Wrapf(err, "error walking to %s", path)
		}

		// (PL) convert \ to / in case of windows path
		path = filepath.ToSlash(path)

		matches, err := a.filter.Matches(path)
		if err != nil {
			return err
		}
		if matches {
			if info.IsDir() && path != source {
				shouldSkip, err := a.filter.MatchesEntireDir(path)
				if err != nil {
					return err
				}
				if shouldSkip {
					return filepath.SkipDir
				}
			}
			return nil
		}

		linkname := ""
		if info.Mode()&os.ModeSymlink != 0 {
			var err error
			linkname, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}

		header, err := tar.FileInfoHeader(info, linkname)
		if err != nil {
			return errors.Wrapf(err, "%s: making header", path)
		}

		clearUIDAndGID(header)

		if sourceIsDir {
			// Name of file in tar should be relative to source directory...
			tmp, err := filepath.Rel(source, path)
			if err != nil {
				return errors.Wrapf(err, "making rel path source:%s path:%s", source, path)
			}
			// ...and live inside `dest`
			header.Name = filepath.Join(dest, tmp)
		} else if strings.HasSuffix(dest, "/") {
			header.Name = dest + filepath.Base(path)
		} else {
			header.Name = dest
		}
		header.Name = filepath.ToSlash(filepath.Clean(header.Name))
		result = append(result, archiveEntry{
			path:   path,
			info:   info,
			header: header,
		})

		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (a *ArchiveBuilder) writeEntry(entry archiveEntry) error {
	path := entry.path
	header := entry.header
	info := entry.info
	err := a.tw.WriteHeader(header)
	if err != nil {
		return errors.Wrapf(err, "%s: writing header", path)
	}

	if info.IsDir() {
		return nil
	}

	if header.Typeflag == tar.TypeReg {
		file, err := os.Open(path)
		if err != nil {
			// In case the file has been deleted since we last looked at it.
			if os.IsNotExist(err) {
				return nil
			}
			return errors.Wrapf(err, "%s: open", path)
		}
		defer func() {
			_ = file.Close()
		}()

		_, err = io.CopyN(a.tw, file, info.Size())
		if err != nil && err != io.EOF {
			return errors.Wrapf(err, "%s: copying Contents", path)
		}
	}
	return nil
}

func tarContextAndUpdateDf(ctx context.Context, writer io.Writer, df dockerfile.Dockerfile, paths []PathMapping, filter model.PathMatcher) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-tarContextAndUpdateDf")
	defer span.Finish()

	ab := NewArchiveBuilder(writer, filter)
	err := ab.ArchivePathsIfExist(ctx, paths)
	if err != nil {
		return errors.Wrap(err, "archivePaths")
	}

	err = ab.archiveDf(ctx, df)
	if err != nil {
		return errors.Wrap(err, "archiveDf")
	}

	return ab.Close()
}

func TarDfOnly(ctx context.Context, writer io.Writer, df dockerfile.Dockerfile) error {
	ab := NewArchiveBuilder(writer, model.EmptyMatcher)
	err := ab.archiveDf(ctx, df)
	if err != nil {
		return errors.Wrap(err, "tarDfOnly")
	}
	return ab.Close()
}

func TarPath(ctx context.Context, writer io.Writer, path string) error {
	ab := NewArchiveBuilder(writer, model.EmptyMatcher)
	err := ab.ArchivePathsIfExist(ctx, []PathMapping{
		{
			LocalPath:     path,
			ContainerPath: ".",
		},
	})
	if err != nil {
		return errors.Wrap(err, "TarPath")
	}

	return ab.Close()
}

func TarArchiveForPaths(ctx context.Context, toArchive []PathMapping, filter model.PathMatcher) io.Reader {
	pr, pw := io.Pipe()
	go tarArchiveForPaths(ctx, pw, toArchive, filter)
	return pr
}

func tarArchiveForPaths(ctx context.Context, pw *io.PipeWriter, toArchive []PathMapping, filter model.PathMatcher) {
	ab := NewArchiveBuilder(pw, filter)
	err := ab.ArchivePathsIfExist(ctx, toArchive)
	if err != nil {
		_ = pw.CloseWithError(errors.Wrap(err, "archivePathsIfExists"))
	} else {
		_ = ab.Close()
		_ = pw.Close()
	}
}

// Dedupe the entries with last-entry-wins semantics.
func dedupeEntries(entries []archiveEntry) []archiveEntry {
	seenIndex := make(map[string]int, len(entries))
	result := make([]archiveEntry, 0, len(entries))
	for i, entry := range entries {
		seenIndex[entry.header.Name] = i
	}
	for i, entry := range entries {
		if seenIndex[entry.header.Name] == i {
			result = append(result, entry)
		}
	}
	return result
}
