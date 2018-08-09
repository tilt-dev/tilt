package temp

import (
	"fmt"
	"os"
	"path/filepath"
)

// An implementation of Dir more suitable
// for directories that need to persist.
type PersistentDir struct {
	dir string
}

func NewPersistentDir(path string) (*PersistentDir, error) {
	_, err := os.Stat(path)
	if err == nil || !os.IsNotExist(err) {
		return nil, fmt.Errorf("NewPersistentDir: dir already exists: %s", path)
	}

	err = os.Mkdir(path, 0777)
	if err != nil {
		return nil, fmt.Errorf("NewPersistentDir failed to create %s", path)
	}

	return &PersistentDir{dir: path}, nil
}

func (d *PersistentDir) NewDir(name string) (Dir, error) {
	path := filepath.Join(d.dir, name)
	return NewPersistentDir(path)
}

func (d *PersistentDir) TearDown() error {
	return os.RemoveAll(d.dir)
}

func (d *PersistentDir) Path() string {
	return d.dir
}
