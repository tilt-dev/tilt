package xdg

import (
	"os"
	"path/filepath"
)

func pathExists(path string) bool {
	fi, err := os.Lstat(path)
	if fi != nil && fi.Mode()&os.ModeSymlink != 0 {
		_, err = filepath.EvalSymlinks(path)
	}

	return err == nil || os.IsExist(err)
}
