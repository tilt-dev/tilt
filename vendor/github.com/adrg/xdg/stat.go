// +build !windows

package xdg

import "os"

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}
