package temp

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/tilt-dev/wmclient/pkg/env"
)

// TempDir holds a temp directory and allows easy access to new temp directories.
type TempDir struct {
	dir string
}

// NewDir creates a new TempDir in the default location (typically $TMPDIR)
func NewDir(prefix string) (*TempDir, error) {
	return NewDirAtRoot("", prefix)
}

// NewDir creates a new TempDir at the given root.
func NewDirAtRoot(root, prefix string) (*TempDir, error) {
	tmpDir, err := ioutil.TempDir(root, prefix)
	if err != nil {
		return nil, fmt.Errorf("temp.NewDir: ioutil.TempDir: %v", err)
	}

	realTmpDir, err := filepath.EvalSymlinks(tmpDir)
	if err != nil {
		return nil, fmt.Errorf("temp.NewDir: filepath.EvalSymlinks: %v", err)
	}

	return &TempDir{dir: realTmpDir}, nil
}

// NewDirAtSlashTmp creates a new TempDir at /tmp
func NewDirAtSlashTmp(prefix string) (*TempDir, error) {
	fullyResolvedPath, err := filepath.EvalSymlinks("/tmp")
	if err != nil {
		return nil, err
	}
	return NewDirAtRoot(fullyResolvedPath, prefix)
}

// d.NewDir creates a new TempDir under d
func (d *TempDir) NewDir(prefix string) (*TempDir, error) {
	d2, err := ioutil.TempDir(d.dir, prefix)
	if err != nil {
		return nil, err
	}
	return &TempDir{d2}, nil
}

func (d *TempDir) NewDeterministicDir(name string) (*TempDir, error) {
	d2 := filepath.Join(d.dir, name)
	err := os.Mkdir(d2, 0700)
	if os.IsExist(err) {
		return nil, err
	} else if err != nil {
		return nil, err
	}
	return &TempDir{d2}, nil
}

func (d *TempDir) TearDown() error {
	if env.IsDebug() {
		return nil
	}
	return os.RemoveAll(d.dir)
}

func (d *TempDir) Path() string {
	return d.dir
}

// Possible extensions:
// temp file
// named directories or files (e.g., we know we want one git repo for our object, but it should be temporary)
