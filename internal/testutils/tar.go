package testutils

import (
	"archive/tar"
	"bytes"
	"io"
	"testing"
)

type ExpectedFile struct {
	Path     string
	Contents string

	// If true, we will assert that the file is not in the container.
	Missing bool
}

func AssertFileInTar(t testing.TB, tr *tar.Reader, expected ExpectedFile) {
	for {
		header, err := tr.Next()
		if err == io.EOF {
			if expected.Missing {
				return
			}
			t.Fatalf("File not found in container: %s", expected.Path)
		} else if err != nil {
			t.Fatalf("Error reading tar file: %v", err)
		}

		if expected.Path == header.Name {
			if expected.Missing {
				t.Errorf("Path %q was not expected in the tarball", expected.Path)
				return
			}

			if header.Typeflag != tar.TypeReg {
				t.Errorf("Path %q exists but is not a regular file", expected.Path)
				return
			}

			contents := bytes.NewBuffer(nil)
			_, err = io.Copy(contents, tr)
			if err != nil {
				t.Fatalf("Error reading tar file: %v", err)
			}

			if contents.String() != expected.Contents {
				t.Errorf("Wrong contents in %q. Expected: %q. Actual: %q",
					expected.Path, expected.Contents, contents.String())
			}
			return // we found it!
		}
	}
}
