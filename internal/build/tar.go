package build

import (
	"archive/tar"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	opentracing "github.com/opentracing/opentracing-go"
)

func archiveDf(ctx context.Context, tw *tar.Writer, df Dockerfile) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-archiveDf")
	defer span.Finish()
	tarHeader := &tar.Header{
		Name: "Dockerfile",
		Size: int64(len(df)),
	}
	err := tw.WriteHeader(tarHeader)
	if err != nil {
		return err
	}
	_, err = tw.Write([]byte(df))
	if err != nil {
		return err
	}

	return nil
}

// archivePaths creates a tar archive of all local files in `paths`. It quietly skips any paths that don't exist.
// NOTE: modifies tw in place.
func archivePaths(ctx context.Context, tw *tar.Writer, paths []pathMapping) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "daemon-archivePaths")
	defer span.Finish()
	for _, p := range paths {
		err := tarPath(ctx, tw, p.LocalPath, p.ContainerPath)
		if err != nil {
			return fmt.Errorf("tarPath '%s': %v", p.LocalPath, err)
		}
	}
	return nil
}

// tarPath writes the given source path into tarWriter at the given dest (recursively for directories).
// e.g. tarring my_dir --> dest d: d/file_a, d/file_b
// If source path does not exist, quietly skips it and returns no err
func tarPath(ctx context.Context, tarWriter *tar.Writer, source, dest string) error {
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

		header, err := tar.FileInfoHeader(info, path)
		if err != nil {
			return fmt.Errorf("%s: making header: %v", path, err)
		}

		if sourceIsDir {
			// Name of file in tar should be relative to source directory...
			header.Name = strings.TrimPrefix(path, source)
		}

		if dest != "" {
			// ...and live inside `dest` (if given)
			header.Name = filepath.Join(dest, header.Name)
		}

		err = tarWriter.WriteHeader(header)
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

			_, err = io.CopyN(tarWriter, file, info.Size())
			if err != nil && err != io.EOF {
				return fmt.Errorf("%s: copying contents: %v", path, err)
			}
		}
		return nil
	})
	return err
}
