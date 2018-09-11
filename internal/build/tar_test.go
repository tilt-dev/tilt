package build

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/windmilleng/tilt/internal/testutils"
)

func TestArchiveDf(t *testing.T) {
	f := newFixture(t)
	ab := newArchiveBuilder()
	defer ab.close()
	defer f.tearDown()

	dfText := "FROM alpine"
	df := Dockerfile(dfText)
	err := ab.archiveDf(f.ctx, df)
	if err != nil {
		t.Fatal(err)
	}

	actual := tar.NewReader(ab.buf)

	f.assertFileInTar(actual, expectedFile{
		path:     "Dockerfile",
		contents: dfText,
	})
}

func TestArchivePathsIfExists(t *testing.T) {
	f := newFixture(t)
	ab := newArchiveBuilder()
	defer ab.close()
	defer f.tearDown()

	f.WriteFile("a", "a")

	paths := []pathMapping{
		pathMapping{
			LocalPath:     f.JoinPath("a"),
			ContainerPath: "/a",
		},
		pathMapping{
			LocalPath:     f.JoinPath("b"),
			ContainerPath: "/b",
		},
	}

	err := ab.archivePathsIfExist(f.ctx, paths)
	if err != nil {
		f.t.Fatal(err)
	}
	actual := tar.NewReader(ab.buf)
	f.assertFileInTar(actual, expectedFile{path: "a", contents: "a"})
	f.assertFileInTar(actual, expectedFile{path: "b", missing: true})
}

func TestLen(t *testing.T) {
	ab := newArchiveBuilder()
	dfText := "FROM alpine"
	df := Dockerfile(dfText)
	err := ab.archiveDf(context.Background(), df)
	if err != nil {
		t.Fatal(err)
	}

	ab.close()
	expected := 2048
	actual := ab.len()

	if actual != expected {
		t.Errorf("Expected size to be %d, got %d", expected, actual)
	}
}

type fixture struct {
	*testutils.TempDirFixture
	t   *testing.T
	ctx context.Context
}

func newFixture(t *testing.T) *fixture {
	ctx := testutils.CtxForTest()

	return &fixture{
		TempDirFixture: testutils.NewTempDirFixture(t),
		t:              t,
		ctx:            ctx,
	}
}

func (f *fixture) assertFileInTar(tr *tar.Reader, expected expectedFile) {
	assertFileInTar(f.t, tr, expected)
}

func (f *fixture) tearDown() {
	f.TempDirFixture.TearDown()
}

func assertFileInTar(t testing.TB, tr *tar.Reader, expected expectedFile) {
	for {
		header, err := tr.Next()
		if err == io.EOF {
			if expected.missing {
				return
			}
			t.Fatalf("File not found in container: %s", expected.path)
		} else if err != nil {
			t.Fatalf("Error reading tar file: %v", err)
		}

		if expected.path == header.Name {
			if expected.missing {
				t.Errorf("Path %q was not expected in the tarball", expected.path)
				return
			}

			if header.Typeflag != tar.TypeReg {
				t.Errorf("Path %q exists but is not a regular file", expected.path)
				return
			}

			contents := bytes.NewBuffer(nil)
			_, err = io.Copy(contents, tr)
			if err != nil {
				t.Fatalf("Error reading tar file: %v", err)
			}

			if contents.String() != expected.contents {
				t.Errorf("Wrong contents in %q. Expected: %q. Actual: %q",
					expected.path, expected.contents, contents.String())
			}
			return // we found it!
		}
	}
}
