package build

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"net"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/tilt-dev/tilt/internal/dockerfile"
	"github.com/tilt-dev/tilt/internal/dockerignore"
	"github.com/tilt-dev/tilt/internal/testutils"
	"github.com/tilt-dev/tilt/internal/testutils/tempdir"
	"github.com/tilt-dev/tilt/pkg/model"
)

func TestArchiveDf(t *testing.T) {
	f := newFixture(t)

	dfText := "FROM alpine"
	buf := new(bytes.Buffer)
	ab := NewArchiveBuilder(buf, model.EmptyMatcher)
	defer ab.Close()

	df := dockerfile.Dockerfile(dfText)
	err := ab.archiveDf(f.ctx, df)
	if err != nil {
		panic(err)
	}

	actual := tar.NewReader(buf)

	f.assertFileInTar(actual, expectedFile{
		Path:                   "Dockerfile",
		Contents:               dfText,
		AssertUidAndGidAreZero: true,
		Mode:                   0644,
	})
}

func TestArchivePathsIfExists(t *testing.T) {
	f := newFixture(t)
	pr, pw := io.Pipe()
	go func() {
		ab := NewArchiveBuilder(pw, model.EmptyMatcher)
		defer ab.Close()

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
		require.NoError(t, err)
		assert.Equal(t, ab.Paths(), []string{f.JoinPath("a")})
	}()

	actual := tar.NewReader(pr)
	f.assertFilesInTar(actual, []expectedFile{
		expectedFile{Path: "a", Contents: "a", AssertUidAndGidAreZero: true, HasExecBitWindows: true},
		expectedFile{Path: "b", Missing: true},
	})
}

func TestDontArchiveTiltfile(t *testing.T) {
	f := newFixture(t)

	filter, err := dockerignore.NewDockerPatternMatcher(f.Path(), []string{"Tiltfile"})
	if err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	ab := NewArchiveBuilder(buf, filter)
	defer ab.Close()

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

	actual := tar.NewReader(buf)

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

func TestArchiveOverlapping(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Cannot create a symlink on windows")
	}

	f := newFixture(t)
	buf := new(bytes.Buffer)
	ab := NewArchiveBuilder(buf, model.EmptyMatcher)
	defer ab.Close()

	f.WriteFile("a/a.txt", "a.txt contents")
	f.WriteFile("b/b.txt", "b.txt contents")
	f.WriteSymlink("../b", "a/b")

	paths := []PathMapping{
		PathMapping{
			LocalPath:     f.JoinPath("a"),
			ContainerPath: "/a",
		},
		PathMapping{
			LocalPath:     f.JoinPath("b"),
			ContainerPath: "/a/b",
		},
	}

	err := ab.ArchivePathsIfExist(f.ctx, paths)
	if err != nil {
		f.t.Fatal(err)
	}

	actual := tar.NewReader(buf)
	f.assertFilesInTar(actual, []expectedFile{
		expectedFile{Path: "a/a.txt", Contents: "a.txt contents", AssertUidAndGidAreZero: true},
		expectedFile{Path: "a/b", IsDir: true},
		expectedFile{Path: "a/b/b.txt", Contents: "b.txt contents"},
	})
}

func TestArchiveSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Cannot create a symlink on windows")
	}

	f := newFixture(t)
	buf := new(bytes.Buffer)
	ab := NewArchiveBuilder(buf, model.EmptyMatcher)
	defer ab.Close()

	f.WriteFile("src/a.txt", "hello world")
	f.WriteSymlink("a.txt", "src/b.txt")

	paths := []PathMapping{
		PathMapping{
			LocalPath:     f.JoinPath("src"),
			ContainerPath: "/src",
		},
	}

	err := ab.ArchivePathsIfExist(f.ctx, paths)
	if err != nil {
		f.t.Fatal(err)
	}

	actual := tar.NewReader(buf)
	f.assertFilesInTar(actual, []expectedFile{
		expectedFile{Path: "src/a.txt", Contents: "hello world"},
		expectedFile{Path: "src/b.txt", Linkname: "a.txt"},
	})
}

func TestArchiveSocket(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Cannot create a unix socket on windows")
	}

	f := newFixture(t)
	buf := new(bytes.Buffer)
	ab := NewArchiveBuilder(buf, model.EmptyMatcher)
	defer ab.Close()

	f.WriteFile("src/a.txt", "hello world")
	c, err := net.Listen("unix", f.JoinPath("src/my.sock"))
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	paths := []PathMapping{
		PathMapping{
			LocalPath:     f.JoinPath("src"),
			ContainerPath: "/src",
		},
	}

	err = ab.ArchivePathsIfExist(f.ctx, paths)
	if err != nil {
		f.t.Fatal(err)
	}

	actual := tar.NewReader(buf)
	f.assertFilesInTar(actual, []expectedFile{
		expectedFile{Path: "src/a.txt", Contents: "hello world"},
		expectedFile{Path: "src/my.sock", Missing: true},
	})
}

func TestArchiveException(t *testing.T) {
	f := newFixture(t)

	filter, err := dockerignore.NewDockerPatternMatcher(f.Path(), []string{"*", "!target"})
	if err != nil {
		t.Fatal(err)
	}

	buf := new(bytes.Buffer)
	ab := NewArchiveBuilder(buf, filter)
	defer ab.Close()

	f.WriteFile("target/foo.txt", "bar")

	paths := []PathMapping{{LocalPath: f.Path(), ContainerPath: "/"}}

	err = ab.ArchivePathsIfExist(f.ctx, paths)
	if err != nil {
		f.t.Fatal(err)
	}

	actual := tar.NewReader(buf)
	f.assertFileInTar(actual, expectedFile{Path: "target/foo.txt", Contents: "bar"})
}

func TestArchiveAllGoFiles(t *testing.T) {
	f := newFixture(t)

	filter, err := dockerignore.NewDockerPatternMatcher(f.Path(),
		[]string{"*", "!pkg/**/*.go"})
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	ab := NewArchiveBuilder(buf, filter)
	defer ab.Close()

	f.WriteFile("pkg/internal/somemodule/foo.go", "bar")

	paths := []PathMapping{{LocalPath: f.Path(), ContainerPath: "/"}}

	err = ab.ArchivePathsIfExist(f.ctx, paths)
	if err != nil {
		f.t.Fatal(err)
	}

	actual := tar.NewReader(buf)
	f.assertFileInTar(actual,
		expectedFile{Path: "pkg/internal/somemodule/foo.go", Contents: "bar"})
}

// Write a file continuously, and make sure we don't get tar errors.
func TestRapidWrite(t *testing.T) {
	f := newFixture(t)

	f.WriteFile("log.txt", "a")

	errCh := make(chan error)
	ctx, cancel := context.WithCancel(f.ctx)
	defer cancel()

	// Continuously open the file for writing.
	go func() {
		defer close(errCh)

		for {
			if ctx.Err() != nil {
				return
			}

			path := f.JoinPath("log.txt")
			f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0755)
			if err != nil {
				errCh <- err
				return
			}

			_, err = f.Write([]byte("a\n"))
			_ = f.Close()
			if err != nil {
				errCh <- err
				return
			}
			time.Sleep(100 * time.Microsecond)
		}
	}()

	paths := []PathMapping{
		PathMapping{
			LocalPath:     f.JoinPath("log.txt"),
			ContainerPath: "/a.txt",
		},
	}

	// Archive the file 10 times and make sure it's a success.
	for i := 0; i < 10; i++ {
		buf := new(bytes.Buffer)
		ab := NewArchiveBuilder(buf, model.EmptyMatcher)
		err := ab.ArchivePathsIfExist(f.ctx, paths)
		require.NoError(t, err)
		assert.Equal(t, ab.Paths(), []string{f.JoinPath("log.txt")})
		require.NoError(t, ab.Close())
		time.Sleep(100 * time.Microsecond)
	}
	cancel()
	assert.NoError(t, <-errCh)
}

type fixture struct {
	*tempdir.TempDirFixture
	t   *testing.T
	ctx context.Context
}

func newFixture(t *testing.T) *fixture {
	ctx, _, _ := testutils.CtxAndAnalyticsForTest()

	return &fixture{
		TempDirFixture: tempdir.NewTempDirFixture(t),
		t:              t,
		ctx:            ctx,
	}
}

func (f *fixture) assertFileInTar(tr *tar.Reader, expected expectedFile) {
	testutils.AssertFileInTar(f.t, tr, expected)
}

func (f *fixture) assertFilesInTar(tr *tar.Reader, expected []expectedFile) {
	testutils.AssertFilesInTar(f.t, tr, expected)
}
