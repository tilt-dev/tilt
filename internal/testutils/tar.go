package testutils

import (
	"archive/tar"
	"bytes"
	"io"
	"testing"
)

const expectedUidAndGid = 0

type ExpectedFile struct {
	Path     string
	Contents string

	// If true, we will assert that the file is not in the tarball.
	Missing bool

	// If true, we will assert that UID and GIF are 0
	AssertUidAndGidAreZero bool
}

// Asserts whether or not this file is in the tar.
func AssertFileInTar(t testing.TB, tr *tar.Reader, expected ExpectedFile) {
	AssertFilesInTar(t, tr, []ExpectedFile{expected})
}

// Asserts whether or not these files are in the tar, but not that they are the only
// files in the tarball.
func AssertFilesInTar(t testing.TB, tr *tar.Reader, expectedFiles []ExpectedFile) {
	burndownMap := make(map[string]ExpectedFile, len(expectedFiles))
	for _, f := range expectedFiles {
		burndownMap[f.Path] = f
	}

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		} else if err != nil {
			t.Fatalf("Error reading tar file: %v", err)
		}

		expected, ok := burndownMap[header.Name]
		if !ok {
			continue
		}

		// we found it!
		delete(burndownMap, expected.Path)

		if expected.Missing {
			t.Errorf("Path %q was not expected in the tarball", expected.Path)
			continue
		}

		if header.Typeflag != tar.TypeReg {
			t.Errorf("Path %q exists but is not a regular file", expected.Path)
			continue
		}

		if expected.AssertUidAndGidAreZero && header.Uid != expectedUidAndGid {
			t.Errorf("Expected %s to have UID 0, got %d", header.Name, header.Uid)
		}

		if expected.AssertUidAndGidAreZero && header.Gid != expectedUidAndGid {
			t.Errorf("Expected %s to have GID 0, got %d", header.Name, header.Gid)
		}

		contents := bytes.NewBuffer(nil)
		_, err = io.Copy(contents, tr)
		if err != nil {
			t.Fatalf("Error reading tar file: %v", err)
		}

		if contents.String() != expected.Contents {
			t.Errorf("Wrong contents in %q. Expected: %q. Actual: %q",
				expected.Path, expected.Contents, contents.String())
			continue
		}

		continue
	}

	for _, f := range burndownMap {
		if !f.Missing {
			t.Errorf("File not found in container: %s", f.Path)
		}
	}
}
