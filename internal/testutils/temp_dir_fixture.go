package testutils

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/windmilleng/wmclient/pkg/os/temp"
)

type TempDirFixture struct {
	t   *testing.T
	dir *temp.TempDir
}

func NewTempDirFixture(t *testing.T) *TempDirFixture {
	dir, err := temp.NewDir(t.Name())
	if err != nil {
		t.Fatalf("Error making temp dir: %v", err)
	}

	return &TempDirFixture{
		t:   t,
		dir: dir,
	}
}

func (f *TempDirFixture) Path() string {
	return f.dir.Path()
}

func (f *TempDirFixture) WriteFile(path string, contents string) {
	fullPath := filepath.Join(f.Path(), path)
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

func (f *TempDirFixture) TouchFiles(paths []string) {
	for _, p := range paths {
		f.WriteFile(p, "")
	}
}

func (f *TempDirFixture) Rm(pathInRepo string) {
	fullPath := filepath.Join(f.Path(), pathInRepo)
	err := os.Remove(fullPath)
	if err != nil {
		f.t.Fatal(err)
	}
}

func (f *TempDirFixture) TearDown() {
	f.dir.TearDown()
}
