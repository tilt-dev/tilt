package testutils

import (
	"archive/tar"
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
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
func AssertFilesInTar(t testing.TB, tr *tar.Reader, expectedFiles []ExpectedFile,
	msgAndArgs ...interface{}) {
	msg := "AssertFilesInTar"
	if len(msgAndArgs) > 0 {
		m := msgAndArgs[0]
		s, ok := m.(string)
		if !ok {
			t.Fatalf("first arg to msgAndArgs (%v) not string", m)
		}
		msg = fmt.Sprintf(s, msgAndArgs[1:]...)
	}
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
			t.Fatalf("Error reading tar file: %v (%s)", err, msg)
		}

		if dupes[header.Name] {
			t.Fatalf("File in tarball twice. This is invalid and will break when extracted: %v (%s)", header.Name, msg)
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
			t.Errorf("Path %q was not expected in the tarball (%s)", expected.Path, msg)
			continue
		}

		expectedReg := !expected.IsDir && expected.Linkname == ""
		expectedDir := expected.IsDir
		expectedSymlink := expected.Linkname != ""
		if expectedReg && header.Typeflag != tar.TypeReg {
			t.Errorf("Path %q exists but is not a regular file (%s)", expected.Path, msg)
			continue
		}

		if expectedDir && header.Typeflag != tar.TypeDir {
			t.Errorf("Path %q exists but is not a directory (%s)", expected.Path, msg)
			continue
		}

		if expectedSymlink && header.Typeflag != tar.TypeSymlink {
			t.Errorf("Path %q exists but is not a directory (%s)", expected.Path, msg)
			continue
		}

		if expected.AssertUidAndGidAreZero && header.Uid != expectedUidAndGid {
			t.Errorf("Expected %s to have UID 0, got %d (%s)", header.Name, header.Uid, msg)
		}

		if expected.AssertUidAndGidAreZero && header.Gid != expectedUidAndGid {
			t.Errorf("Expected %s to have GID 0, got %d (%s)", header.Name, header.Gid, msg)
		}

		if header.Linkname != expected.Linkname {
			t.Errorf("Expected linkname %q, actual %q (%s)", expected.Linkname, header.Linkname, msg)
		}

		if expectedReg {
			contents := bytes.NewBuffer(nil)
			_, err = io.Copy(contents, tr)
			if err != nil {
				t.Fatalf("Error reading tar file: %v (%s)", err, msg)
			}

			if !assert.Equal(t, expected.Contents, contents.String()) {
				fmt.Printf("wrong contents in %q\n (%s)", expected.Path, msg)
				continue
			}
		}

		continue
	}

	for _, f := range burndownMap {
		if !f.Missing {
			t.Errorf("File not found in container: %s (%s)", f.Path, msg)
		}
	}
}
