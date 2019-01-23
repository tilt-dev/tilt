package tempdir

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/windmilleng/wmclient/pkg/os/temp"
)

type TempDirFixture struct {
	t   testing.TB
	dir *temp.TempDir
}

func NewTempDirFixture(t testing.TB) *TempDirFixture {
	name := t.Name()
	name = strings.Replace(name, "/", "-", -1)
	dir, err := temp.NewDir(name)
	if err != nil {
		t.Fatalf("Error making temp dir: %v", err)
	}

	return &TempDirFixture{
		t:   t,
		dir: dir,
	}
}

func (f *TempDirFixture) T() testing.TB {
	return f.t
}

func (f *TempDirFixture) Path() string {
	return f.dir.Path()
}

func (f *TempDirFixture) JoinPath(path ...string) string {
	p := []string{}
	isAbs := len(path) > 0 && filepath.IsAbs(path[0])
	if isAbs {
		if !strings.HasPrefix(path[0], f.Path()) {
			f.t.Fatalf("Path outside fixture tempdir are forbidden: %s", path[0])
		}
	} else {
		p = append(p, f.Path())
	}

	p = append(p, path...)
	return filepath.Join(p...)
}

func (f *TempDirFixture) JoinPaths(paths []string) []string {
	joined := make([]string, len(paths))
	for i, p := range paths {
		joined[i] = f.JoinPath(p)
	}
	return joined
}

func (f *TempDirFixture) WriteFile(path string, contents string) {
	fullPath := f.JoinPath(path)
	base := filepath.Dir(fullPath)
	err := os.MkdirAll(base, os.FileMode(0777))
	if err != nil {
		f.t.Fatal(err)
	}
	err = ioutil.WriteFile(fullPath, []byte(contents), os.FileMode(0777))
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *TempDirFixture) WriteSymlink(srcPath, destPath string) {
	fullSrcPath := f.JoinPath(srcPath)
	fullDestPath := f.JoinPath(destPath)
	err := os.MkdirAll(filepath.Dir(fullDestPath), os.FileMode(0777))
	if err != nil {
		f.t.Fatal(err)
	}
	err = os.Symlink(fullSrcPath, fullDestPath)
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *TempDirFixture) MkdirAll(path string) {
	fullPath := f.JoinPath(path)
	err := os.MkdirAll(fullPath, os.FileMode(0777))
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *TempDirFixture) TouchFiles(paths []string) {
	for _, p := range paths {
		f.WriteFile(p, "")
	}
}

func (f *TempDirFixture) Rm(pathInRepo string) {
	fullPath := f.JoinPath(pathInRepo)
	err := os.RemoveAll(fullPath)
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *TempDirFixture) NewFile(prefix string) (*os.File, error) {
	return ioutil.TempFile(f.dir.Path(), prefix)
}

func (f *TempDirFixture) TempDir(prefix string) string {
	name, err := ioutil.TempDir(f.dir.Path(), prefix)
	if err != nil {
		f.t.Fatal(err)
	}
	return name
}

func (f *TempDirFixture) TearDown() {
	err := f.dir.TearDown()
	if err != nil {
		f.t.Fatal(err)
	}
}
