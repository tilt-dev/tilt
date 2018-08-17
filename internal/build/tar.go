package build

import (
	"archive/tar"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func archiveDf(tw *tar.Writer, df string) error {
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

// archiveIfExists creates a tar archive of all local files in `paths` _if they exist_,
// and returns a list of all PathMappings that point to nonexistent LocalPaths (we'll
// use that later to add RM statements).
// NOTE: modifies tw in place.
func archiveIfExists(tw *tar.Writer, paths []pathMapping) (dnePaths []pathMapping, err error) {
	for _, p := range paths {
		dne, err := tarPath(tw, p.LocalPath, p.ContainerPath)
		if err != nil {
			return nil, fmt.Errorf("tarPath '%s': %v", p.LocalPath, err)
		}
		if dne {
			// Tried to tar path but it didn't exist -- expected error, add it to list of nonexistent paths
			dnePaths = append(dnePaths, p)
		}
	}
	return dnePaths, nil
}

// tarPath writes the given source path into tarWriter at the given dest (recursively for directories).
// e.g. tarring my_dir --> dest d: d/file_a, d/file_b
// If source path does not exist, returns `doesNotExist = true` and no err (DNE is an expected error)
func tarPath(tarWriter *tar.Writer, source, dest string) (doesNotExist bool, err error) {
	sourceInfo, err := os.Stat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, fmt.Errorf("%s: stat: %v", source, err)
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
				return fmt.Errorf("%s: open: %v", path, err)
			}
			defer file.Close()

			_, err = io.CopyN(tarWriter, file, info.Size())
			if err != nil && err != io.EOF {
				return fmt.Errorf("%s: copying contents: %v", path, err)
			}
		}
		return nil
	})
	return false, err
}
