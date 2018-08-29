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

	actual := ab.buf

	f.assertFileInTarWithContents(actual, "Dockerfile", dfText)
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
	actual := ab.buf
	f.assertFileInTarWithContents(actual, "a", "a")
	f.assertFileNotInTar(actual, "b")
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

func (f *fixture) tearDown() {
	f.TempDirFixture.TearDown()
}

func (f *fixture) assertFileInTarWithContents(buf *bytes.Buffer, path, expected string) {
	tr := tar.NewReader(buf)
	found := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}

		if err != nil {
			f.t.Fatal(err)
		}

		if hdr.Name == path {
			found = true
			buf := new(bytes.Buffer)
			buf.ReadFrom(tr)
			if buf.String() != expected {
				f.t.Errorf("Expected %s to equal %s", buf.String(), expected)
			}
		}
	}

	if !found {
		f.t.Errorf("Expected to find a file at %s, but no such file was found", path)
	}
}

func (f *fixture) assertFileNotInTar(buf *bytes.Buffer, path string) {
	tr := tar.NewReader(buf)
	found := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break // End of archive
		}

		if err != nil {
			f.t.Fatal(err)
		}

		if hdr.Name == path {
			found = true
			break
		}
	}

	if found {
		f.t.Errorf("Expected not to find a file at %s, but file was found", path)
	}
}
