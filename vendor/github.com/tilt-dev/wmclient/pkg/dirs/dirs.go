package dirs

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/mitchellh/go-homedir"
)

// TiltDevDir returns the root settings directory.
// For legacy reasons, we use ~/.windmill if it exists.
// Otherwise, we use ~/.tilt-dev
func GetTiltDevDir() (string, error) {
	dir := os.Getenv("TILT_DEV_DIR")
	if dir == "" {
		dir = os.Getenv("WMDAEMON_HOME")
	}
	if dir == "" {
		dir = os.Getenv("WINDMILL_DIR")
	}
	if dir == "" {
		homedir, err := homedir.Dir()
		if err != nil {
			return "", err
		}

		if homedir == "" {
			return "", fmt.Errorf("Cannot find home directory; $HOME unset")
		}
		dir = filepath.Join(homedir, ".windmill")
		if _, err := os.Stat(dir); err != nil {
			if os.IsNotExist(err) {
				dir = filepath.Join(homedir, ".tilt-dev")
			} else if err != nil {
				return "", err
			}
		}

	}

	dir, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}

	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			os.Mkdir(dir, os.FileMode(0755))
		} else {
			return "", err
		}
	}

	return dir, nil
}

func UseTiltDevDir() (*TiltDevDir, error) {
	dir, err := GetTiltDevDir()
	if err != nil {
		return nil, err
	}

	return &TiltDevDir{dir: dir}, nil
}

// Create a .tilt-dev dir at an arbitrary directory. Useful for testing.
func NewTiltDevDirAt(dir string) *TiltDevDir {
	return &TiltDevDir{dir: dir}
}

type TiltDevDir struct {
	dir string
}

func (d *TiltDevDir) Root() string {
	return d.dir
}

func (d *TiltDevDir) OpenFile(p string, flag int, perm os.FileMode) (*os.File, error) {
	err := d.MkdirAll(filepath.Dir(p))
	if err != nil {
		return nil, err
	}

	return os.OpenFile(filepath.Join(d.dir, p), flag, perm)
}

func (d *TiltDevDir) WriteFile(p, text string) error {
	err := d.MkdirAll(filepath.Dir(p))
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(d.dir, p), []byte(text), os.FileMode(0700))
}

func (d *TiltDevDir) ReadFile(p string) (string, error) {
	if filepath.IsAbs(p) {
		return "", fmt.Errorf("TiltDevDir.ReadFile: p must be relative to .tilt-dev root: %v", p)
	}

	abs := filepath.Join(d.dir, p)
	bs, err := ioutil.ReadFile(abs)
	if err != nil {
		return "", err
	}

	return string(bs), nil
}

func (d *TiltDevDir) MkdirAll(p string) error {
	if filepath.IsAbs(p) {
		return fmt.Errorf("TiltDevDir.MkdirAll: p must be relative to .tilt-dev root: %v", p)
	}

	return os.MkdirAll(filepath.Join(d.dir, p), os.FileMode(0700))
}

func (d *TiltDevDir) Abs(p string) (string, error) {
	if filepath.IsAbs(p) {
		return "", fmt.Errorf("TiltDevDir.Abs: p must be relative to .tilt-dev root: %v", p)
	}

	return filepath.Join(d.dir, p), nil
}
