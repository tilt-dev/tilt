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

	// If true, we will assert the file is a dir.
	IsDir bool

	// If true, we will assert that UID and GID are 0
	AssertUidAndGidAreZero bool

	// If true, we will assert that this is a symlink with a linkname.
	Linkname string
}

// Asserts whether or not this file is in the tar.
func AssertFileInTar(t testing.TB, tr *tar.Reader, expected ExpectedFile) {
	AssertFilesInTar(t, tr, []ExpectedFile{expected})
}

// Asserts whether or not these files are in the tar, but not that they are the only
// files in the tarball.
func AssertFilesInTar(t testing.TB, tr *tar.Reader, expectedFiles []ExpectedFile) {
	dupes := make(map[string]bool)

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

		if dupes[header.Name] {
			t.Fatalf("File in tarball twice. This is invalid and will break when extracted: %v", header.Name)
			break
		}

		dupes[header.Name] = true

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

		expectedReg := !expected.IsDir && expected.Linkname == ""
		expectedDir := expected.IsDir
		expectedSymlink := expected.Linkname != ""
		if expectedReg && header.Typeflag != tar.TypeReg {
			t.Errorf("Path %q exists but is not a regular file", expected.Path)
			continue
		}

		if expectedDir && header.Typeflag != tar.TypeDir {
			t.Errorf("Path %q exists but is not a directory", expected.Path)
			continue
		}

		if expectedSymlink && header.Typeflag != tar.TypeSymlink {
			t.Errorf("Path %q exists but is not a symlink", expected.Path)
			continue
		}

		if expected.AssertUidAndGidAreZero && header.Uid != expectedUidAndGid {
			t.Errorf("Expected %s to have UID 0, got %d", header.Name, header.Uid)
		}

		if expected.AssertUidAndGidAreZero && header.Gid != expectedUidAndGid {
			t.Errorf("Expected %s to have GID 0, got %d", header.Name, header.Gid)
		}

		if header.Linkname != expected.Linkname {
			t.Errorf("Expected linkname %q, actual %q", expected.Linkname, header.Linkname)
		}

		if expectedReg {
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
		}

		continue
	}

	for _, f := range burndownMap {
		if !f.Missing {
			t.Errorf("File not found in container: %s", f.Path)
		}
	}
}
