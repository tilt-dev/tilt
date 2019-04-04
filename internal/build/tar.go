package build

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/windmilleng/tilt/internal/dockerfile"
	"github.com/windmilleng/tilt/internal/model"

	"github.com/karrick/godirwalk"
	opentracing "github.com/opentracing/opentracing-go"
)

type ArchiveBuilder struct {
	tw     *tar.Writer
	buf    *bytes.Buffer
	filter model.PathMatcher
	paths  []string // local paths archived
}

func NewArchiveBuilder(filter model.PathMatcher) *ArchiveBuilder {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)
	if filter == nil {
		filter = model.EmptyMatcher
	}

	return &ArchiveBuilder{tw: tw, buf: buf, filter: filter}
}

func (a *ArchiveBuilder) close() error {
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
	// It's not obvious that this is the correct behavor. A better approach
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

func (a *ArchiveBuilder) BytesBuffer() (*bytes.Buffer, error) {
	err := a.close()
	if err != nil {
		return nil, err
	}

	return a.buf, nil
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
	sourceInfo, err := os.Stat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, errors.Wrapf(err, "%s: stat", source)
	}

	sourceIsDir := sourceInfo.IsDir()
	result := make([]archiveEntry, 0)
	if !sourceIsDir {
		matches, err := a.filter.Matches(source, false)
		if err != nil {
			return nil, err
		} else if matches {
			return result, err
		}

		linkname := ""
		if sourceInfo.Mode()&os.ModeSymlink != 0 {
			var err error
			linkname, err = os.Readlink(source)
			if err != nil {
				return result, err
			}
		}

		header, err := tar.FileInfoHeader(sourceInfo, linkname)
		clearUIDAndGID(header)
		if err != nil {
			return result, errors.Wrapf(err, "%s: making header", source)
		}

		if strings.HasSuffix(dest, string(filepath.Separator)) {
			header.Name = filepath.Join(dest, filepath.Base(source))
		} else {
			header.Name = dest
		}

		header.Name = filepath.Clean(header.Name)
		result = append(result, archiveEntry{
			path:   source,
			info:   sourceInfo,
			header: header,
		})

		return result, nil
	} else {
		// Make sure we can trim this off filenames to get valid relative filepaths
		if !strings.HasSuffix(source, "/") {
			source += "/"
		}
	}

	dest = strings.TrimPrefix(dest, "/")

	err = godirwalk.Walk(source, &godirwalk.Options{
		Callback: func(path string, de *godirwalk.Dirent) error {
			matches, err := a.filter.Matches(path, de.IsDir())
			if err != nil {
				return err
			} else if matches {
				return nil
			}

			info, err := os.Lstat(path)
			if err != nil {
				return err
			}

			linkname := ""
			if info.Mode()&os.ModeSymlink != 0 {
				var err error
				linkname, err = os.Readlink(path)
				if err != nil {
					return err
				}

				symStat, err := os.Stat(path)
				if err != nil {
					return err
				}
				if symStat.IsDir() {
					info = symStat
				}
			}

			header, err := tar.FileInfoHeader(info, linkname)
			clearUIDAndGID(header)
			if err != nil {
				return errors.Wrapf(err, "%s: making header", path)
			}

			if sourceIsDir {
				// Name of file in tar should be relative to source directory...
				header.Name = strings.TrimPrefix(path, source)
				// ...and live inside `dest`
				header.Name = filepath.Join(dest, header.Name)
			} else if strings.HasSuffix(dest, string(filepath.Separator)) {
				header.Name = filepath.Join(dest, filepath.Base(source))
			} else {
				header.Name = dest
			}

			header.Name = filepath.Clean(header.Name)
			result = append(result, archiveEntry{
				path:   path,
				info:   info,
				header: header,
			})

			return nil
		},
		Unsorted: true, // (optional) set true for faster yet non-deterministic enumeration (see godoc)
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

func (a *ArchiveBuilder) len() int {
	return a.buf.Len()
}

func tarContextAndUpdateDf(ctx context.Context, df dockerfile.Dockerfile, paths []PathMapping, filter model.PathMatcher) (*bytes.Buffer, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-tarContextAndUpdateDf")
	defer span.Finish()

	ab := NewArchiveBuilder(filter)
	err := ab.ArchivePathsIfExist(ctx, paths)
	if err != nil {
		return nil, errors.Wrap(err, "archivePaths")
	}

	err = ab.archiveDf(ctx, df)
	if err != nil {
		return nil, errors.Wrap(err, "archiveDf")
	}

	return ab.BytesBuffer()
}

func tarDfOnly(ctx context.Context, df dockerfile.Dockerfile) (*bytes.Buffer, error) {
	ab := NewArchiveBuilder(model.EmptyMatcher)
	err := ab.archiveDf(ctx, df)
	if err != nil {
		return nil, errors.Wrap(err, "tarDfOnly")
	}
	return ab.BytesBuffer()
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
