package dirs

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
)

func CurrentHomeDir() (string, error) {
	home := os.Getenv("HOME")
	if home != "" {
		return home, nil
	}

	current, err := user.Current()
	if err != nil {
		return "", err
	}
	return current.HomeDir, nil
}

// WindmillDir returns the root Windmill directory; by default ~/.windmill
func GetWindmillDir() (string, error) {
	dir := os.Getenv("WMDAEMON_HOME")
	if dir == "" {
		dir = os.Getenv("WINDMILL_DIR")
	}
	if dir == "" {
		homedir, err := CurrentHomeDir()
		if err != nil {
			return "", err
		}

		if homedir == "" {
			return "", fmt.Errorf("Cannot find home directory; $HOME unset")
		}
		dir = filepath.Join(homedir, ".windmill")
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

func UseWindmillDir() (*WindmillDir, error) {
	dir, err := GetWindmillDir()
	if err != nil {
		return nil, err
	}

	return &WindmillDir{dir: dir}, nil
}

// Create a windmill dir at an arbitrary directory. Useful for testing.
func NewWindmillDirAt(dir string) *WindmillDir {
	return &WindmillDir{dir: dir}
}

type WindmillDir struct {
	dir string
}

func (d *WindmillDir) Root() string {
	return d.dir
}

func (d *WindmillDir) OpenFile(p string, flag int, perm os.FileMode) (*os.File, error) {
	err := d.MkdirAll(filepath.Dir(p))
	if err != nil {
		return nil, err
	}

	return os.OpenFile(filepath.Join(d.dir, p), flag, perm)
}

func (d *WindmillDir) WriteFile(p, text string) error {
	err := d.MkdirAll(filepath.Dir(p))
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filepath.Join(d.dir, p), []byte(text), os.FileMode(0700))
}

func (d *WindmillDir) ReadFile(p string) (string, error) {
	if filepath.IsAbs(p) {
		return "", fmt.Errorf("WindmillDir.ReadFile: p must be relative to .windmill root: %v", p)
	}

	abs := filepath.Join(d.dir, p)
	bs, err := ioutil.ReadFile(abs)
	if err != nil {
		return "", err
	}

	return string(bs), nil
}

func (d *WindmillDir) MkdirAll(p string) error {
	if filepath.IsAbs(p) {
		return fmt.Errorf("WindmillDir.MkdirAll: p must be relative to .windmill root: %v", p)
	}

	return os.MkdirAll(filepath.Join(d.dir, p), os.FileMode(0700))
}

func (d *WindmillDir) Abs(p string) (string, error) {
	if filepath.IsAbs(p) {
		return "", fmt.Errorf("WindmillDir.Abs: p must be relative to .windmill root: %v", p)
	}

	return filepath.Join(d.dir, p), nil
}
