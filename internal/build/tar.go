package build

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/windmilleng/tilt/internal/tiltfile"

	opentracing "github.com/opentracing/opentracing-go"
)

type ArchiveBuilder struct {
	tw  *tar.Writer
	buf *bytes.Buffer
}

func NewArchiveBuilder() *ArchiveBuilder {
	buf := new(bytes.Buffer)
	tw := tar.NewWriter(buf)

	return &ArchiveBuilder{tw: tw, buf: buf}
}

func (a *ArchiveBuilder) close() error {
	return a.tw.Close()
}

func (a *ArchiveBuilder) archiveDf(ctx context.Context, df Dockerfile) error {
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
func (a *ArchiveBuilder) ArchivePathsIfExist(ctx context.Context, paths []pathMapping) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-ArchivePathsIfExist")
	defer span.Finish()
	for _, p := range paths {
		err := a.tarPath(ctx, p.LocalPath, p.ContainerPath)
		if err != nil {
			return fmt.Errorf("tarPath '%s': %v", p.LocalPath, err)
		}
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

func pathIsTiltfile(p string) bool {
	_, f := path.Split(p)
	return f == tiltfile.FileName
}

// tarPath writes the given source path into tarWriter at the given dest (recursively for directories).
// e.g. tarring my_dir --> dest d: d/file_a, d/file_b
// If source path does not exist, quietly skips it and returns no err
func (a *ArchiveBuilder) tarPath(ctx context.Context, source, dest string) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, fmt.Sprintf("daemon-tarPath-%s", source))
	span.SetTag("source", source)
	span.SetTag("dest", dest)
	defer span.Finish()
	sourceInfo, err := os.Stat(source)

	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("%s: stat: %v", source, err)
	}

	sourceIsDir := sourceInfo.IsDir()
	if sourceIsDir {
		// Make sure we can trim this off filenames to get valid relative filepaths
		if !strings.HasSuffix(source, "/") {
			source += "/"
		}
	}

	dest = strings.TrimPrefix(dest, "/")

	err = filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error walking to %s: %v", path, err)
		}

		if pathIsTiltfile(path) {
			return nil
		}

		header, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return fmt.Errorf("%s: making header: %v", path, err)
		}

		if sourceIsDir {
			// Name of file in tar should be relative to source directory...
			header.Name = strings.TrimPrefix(path, source)
			// ...and live inside `dest`
			header.Name = filepath.Join(dest, header.Name)
		} else {
			header.Name = dest
		}

		err = a.tw.WriteHeader(header)
		if err != nil {
			return fmt.Errorf("%s: writing header: %v", path, err)
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
				return fmt.Errorf("%s: open: %v", path, err)
			}
			defer func() {
				_ = file.Close()
			}()

			_, err = io.CopyN(a.tw, file, info.Size())
			if err != nil && err != io.EOF {
				return fmt.Errorf("%s: copying Contents: %v", path, err)
			}
		}
		return nil
	})
	return err
}

func (a *ArchiveBuilder) len() int {
	return a.buf.Len()
}

func tarContextAndUpdateDf(ctx context.Context, df Dockerfile, paths []pathMapping) (*bytes.Buffer, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-tarContextAndUpdateDf")
	defer span.Finish()

	ab := NewArchiveBuilder()
	err := ab.ArchivePathsIfExist(ctx, paths)
	if err != nil {
		return nil, fmt.Errorf("archivePaths: %v", err)
	}

	err = ab.archiveDf(ctx, df)
	if err != nil {
		return nil, fmt.Errorf("archiveDf: %v", err)
	}

	return ab.BytesBuffer()
}
