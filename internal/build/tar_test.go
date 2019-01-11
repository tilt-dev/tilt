package build

import (
	"archive/tar"
	"context"
	"testing"

	"github.com/windmilleng/tilt/internal/dockerfile"
	"github.com/windmilleng/tilt/internal/dockerignore"
	"github.com/windmilleng/tilt/internal/model"
	"github.com/windmilleng/tilt/internal/testutils"
	"github.com/windmilleng/tilt/internal/testutils/output"
	"github.com/windmilleng/tilt/internal/testutils/tempdir"
)

func TestArchiveDf(t *testing.T) {
	f := newFixture(t)
	ab := NewArchiveBuilder(model.EmptyMatcher)
	defer ab.close()
	defer f.tearDown()

	dfText := "FROM alpine"
	df := dockerfile.Dockerfile(dfText)
	err := ab.archiveDf(f.ctx, df)
	if err != nil {
		t.Fatal(err)
	}

	actual := tar.NewReader(ab.buf)

	f.assertFileInTar(actual, expectedFile{
		Path:                   "Dockerfile",
		Contents:               dfText,
		AssertUidAndGidAreZero: true,
	})
}

func TestArchivePathsIfExists(t *testing.T) {
	f := newFixture(t)
	ab := NewArchiveBuilder(model.EmptyMatcher)
	defer ab.close()
	defer f.tearDown()

	f.WriteFile("a", "a")

	paths := []PathMapping{
		PathMapping{
			LocalPath:     f.JoinPath("a"),
			ContainerPath: "/a",
		},
		PathMapping{
			LocalPath:     f.JoinPath("b"),
			ContainerPath: "/b",
		},
	}

	err := ab.ArchivePathsIfExist(f.ctx, paths)
	if err != nil {
		f.t.Fatal(err)
	}
	actual := tar.NewReader(ab.buf)
	f.assertFileInTar(actual, expectedFile{Path: "a", Contents: "a", AssertUidAndGidAreZero: true})
	f.assertFileInTar(actual, expectedFile{Path: "b", Missing: true})
}

func TestLen(t *testing.T) {
	ab := NewArchiveBuilder(model.EmptyMatcher)
	dfText := "FROM alpine"
	df := dockerfile.Dockerfile(dfText)
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

func TestDontArchiveTiltfile(t *testing.T) {
	f := newFixture(t)
	defer f.tearDown()

	filter, err := dockerignore.NewDockerPatternMatcher(f.Path(), []string{"Tiltfile"})
	if err != nil {
		t.Fatal(err)
	}

	ab := NewArchiveBuilder(filter)
	defer ab.close()

	f.WriteFile("a", "a")
	f.WriteFile("Tiltfile", "Tiltfile")

	paths := []PathMapping{
		PathMapping{
			LocalPath:     f.JoinPath("a"),
			ContainerPath: "/a",
		},
		PathMapping{
			LocalPath:     f.JoinPath("Tiltfile"),
			ContainerPath: "/Tiltfile",
		},
	}

	err = ab.ArchivePathsIfExist(f.ctx, paths)
	if err != nil {
		f.t.Fatal(err)
	}
	actual := tar.NewReader(ab.buf)

	testutils.AssertFilesInTar(
		t,
		actual,
		[]testutils.ExpectedFile{
			testutils.ExpectedFile{
				Path:     "a",
				Contents: "a",
			},
			testutils.ExpectedFile{
				Path:    "Tiltfile",
				Missing: true,
			},
		},
	)
}

type fixture struct {
	*tempdir.TempDirFixture
	t   *testing.T
	ctx context.Context
}

func newFixture(t *testing.T) *fixture {
	ctx := output.CtxForTest()

	return &fixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
		t:              t,
		ctx:            ctx,
	}
}

func (f *fixture) assertFileInTar(tr *tar.Reader, expected expectedFile) {
	testutils.AssertFileInTar(f.t, tr, expected)
}

func (f *fixture) tearDown() {
	f.TempDirFixture.TearDown()
}
